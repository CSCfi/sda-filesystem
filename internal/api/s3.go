package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"sda-filesystem/internal/logs"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyendpoints "github.com/aws/smithy-go/endpoints"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/neicnordic/crypt4gh/streaming"
)

// Crypt4GH constants
const BlockSize int64 = 65536
const MacSize int64 = 28
const CipherBlockSize = BlockSize + MacSize

// chunkSize is the size of a single request when requesting object content from storage
const chunkSize = 1 << 25

// Metadata standardises the metadata received for both buckets and objects
type Metadata struct {
	Name         string
	Size         int64
	LastModified *time.Time
}

type resolverV2 struct{}
type EndpointKey struct{} // custom key for checking context value when resolving endpoint

// ResolveEndpoint adds a prefix to the s3 endpoint based on the value in context.
// This enables us to to use the same s3 client to call both SD Connect and SD Apply endpoints.
func (*resolverV2) ResolveEndpoint(ctx context.Context, params s3.EndpointParameters) (
	smithyendpoints.Endpoint, error,
) {
	if e := ctx.Value(EndpointKey{}); e != nil {
		smithyEndpoint, err := s3.NewDefaultEndpointResolverV2().ResolveEndpoint(ctx, params)
		if err == nil {
			smithyEndpoint.URI.Path = e.(string) + smithyEndpoint.URI.Path
		}

		return smithyEndpoint, err
	}

	return smithyendpoints.Endpoint{}, errors.New("endpoint context not valid")
}

// objectToQuery adds object name as a query parameter in certain requests
var objectToQuery = middleware.SerializeMiddlewareFunc("objectToQuery", func(
	ctx context.Context, in middleware.SerializeInput, next middleware.SerializeHandler,
) (
	out middleware.SerializeOutput, metadata middleware.Metadata, err error,
) {
	var key string
	switch v := in.Parameters.(type) {
	case *s3.GetObjectInput:
		key = *v.Key
	case *s3.PutObjectInput:
		key = *v.Key
	case *s3.CreateMultipartUploadInput:
		key = *v.Key
	case *s3.UploadPartInput:
		key = *v.Key
	case *s3.AbortMultipartUploadInput:
		key = *v.Key
	case *s3.CompleteMultipartUploadInput:
		key = *v.Key
	case *s3.DeleteObjectInput:
		key = *v.Key
	}

	if key != "" {
		req := in.Request.(*smithyhttp.Request)
		q := req.URL.Query()
		q.Add("object", key)
		req.URL.RawQuery = q.Encode()
	}

	return next.HandleSerialize(ctx, in)
})

// methodHeadToGet forces the request method to be GET if it was originally HEAD
var methodHeadToGet = middleware.BuildMiddlewareFunc("methodHeadToGet", func(
	ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
) (
	out middleware.BuildOutput, metadata middleware.Metadata, err error,
) {
	req := in.Request.(*smithyhttp.Request)
	if req.Method == "HEAD" {
		req.Method = "GET"
	}

	return next.HandleBuild(ctx, in)
})

// customFinalize adds necessary headers to request so that the user can be authenticated
// in KrakenD, and removes object from URL path if it is there.
var customFinalize = middleware.FinalizeMiddlewareFunc("customFinalize", func(
	ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler,
) (
	out middleware.FinalizeOutput, metadata middleware.Metadata, err error,
) {
	req := in.Request.(*smithyhttp.Request)

	// Remove object from URL path (we want it as a query parameter)
	q := req.URL.Query()
	object := q.Get("object")
	if object != "" {
		req.URL.Path = strings.TrimSuffix(req.URL.Path, object)
	}

	// Override Authorization header set by aws
	req.Header.Set("Authorization", "Bearer "+ai.userProfile.DesktopToken)
	if ai.password != "" {
		req.Header.Set("CSC-Password", ai.password)
	}

	return next.HandleFinalize(ctx, in)
})

var initialiseS3Client = func() error {
	tr := ai.hi.client.Transport.(*http.Transport).Clone()
	tr.MaxConnsPerHost = 100
	tr.MaxIdleConnsPerHost = 100

	httpClient := &http.Client{
		Transport: tr,
		Timeout:   time.Second * time.Duration(ai.hi.s3Timeout),
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("auto"), // Will be replaced in KrakenD
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
		config.WithHTTPClient(httpClient),
		config.WithRetryer(func() aws.Retryer {
			return retry.AddWithMaxAttempts(retry.NewStandard(), ai.hi.httpRetry)
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to load S3 config: %w", err)
	}

	cfg.APIOptions = append(cfg.APIOptions, func(stack *middleware.Stack) error {
		return stack.Serialize.Add(objectToQuery, middleware.After)
	})
	cfg.APIOptions = append(cfg.APIOptions, func(stack *middleware.Stack) error {
		return stack.Build.Add(methodHeadToGet, middleware.After)
	})
	cfg.APIOptions = append(cfg.APIOptions, func(stack *middleware.Stack) error {
		return stack.Finalize.Add(customFinalize, middleware.After)
	})

	ai.hi.s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(ai.proxy)
		o.UsePathStyle = true
		o.EndpointResolverV2 = &resolverV2{}
		o.DisableLogOutputChecksumValidationSkipped = true
	})

	logs.Debug("Initialised S3 client")

	return nil
}

func getContext(rep string, head bool) context.Context {
	endpoint := ai.hi.endpoints.S3.Default
	if head {
		endpoint = ai.hi.endpoints.S3.Head
	}

	return context.WithValue(context.Background(), EndpointKey{}, endpoint+strings.ToLower(rep))
}

// BucketExists checks whether or not bucket already exists in S3 storage
var BucketExists = func(rep, bucket string) (bool, error) {
	ctx := getContext(rep, true)

	_, err := ai.hi.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		var re *smithyhttp.ResponseError
		if errors.As(err, &re) {
			if re.HTTPStatusCode() == 404 {
				return false, nil
			} else if re.HTTPStatusCode() == 403 {
				return true, fmt.Errorf("bucket %s is already in use by another project", bucket)
			}

			err = re.Err
		}

		return true, fmt.Errorf("could not use bucket %s: %w", bucket, err)
	}

	logs.Infof("Bucket %s exists", bucket)

	return true, nil
}

// CreateBucket creates bucket and waits for it to be ready for subsequent requests
var CreateBucket = func(rep, bucket string) error {
	ctx := getContext(rep, false)

	_, err := ai.hi.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		var re *smithyhttp.ResponseError
		if errors.As(err, &re) {
			err = re.Err
		}

		return fmt.Errorf("could not create bucket %s: %w", bucket, err)
	}

	ctx = getContext(rep, true)
	err = s3.NewBucketExistsWaiter(ai.hi.s3Client).Wait(
		ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)}, time.Minute)
	if err != nil {
		var re *smithyhttp.ResponseError
		if errors.As(err, &re) {
			err = re.Err
		}

		return fmt.Errorf("failed to wait for bucket %s to exist: %w", bucket, err)
	}

	return nil
}

// GetBuckets returns metadata for all the buckets in a certain repository
var GetBuckets = func(rep string) ([]Metadata, error) {
	params := &s3.ListBucketsInput{}
	paginator := s3.NewListBucketsPaginator(ai.hi.s3Client, params, func(o *s3.ListBucketsPaginatorOptions) {
		o.Limit = 1000 // Allas does not currently support this and returns all buckets in one request
	})

	ctx := getContext(rep, false)

	var buckets []types.Bucket
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			var re *smithyhttp.ResponseError
			if errors.As(err, &re) {
				err = re.Err
			}

			return nil, fmt.Errorf("failed to list buckets for %s: %w", ToPrint(rep), err)
		}

		buckets = append(buckets, output.Buckets...)
	}

	if len(buckets) == 0 {
		return nil, fmt.Errorf("no buckets found for %s", ToPrint(rep))
	}

	logs.Infof("Retrieved buckets for %s", ToPrint(rep))

	meta := make([]Metadata, len(buckets))
	for i := range meta {
		// Size and modification time of bucket will be calculated later based on the objects it contains
		meta[i] = Metadata{*buckets[i].Name, 0, nil}
	}

	return meta, nil
}

// GetObjects returns metadata for all the objects in a particular bucket.
// `prefix` is an optional parameter with which function can return only objects that
// begin with that particular value.
var GetObjects = func(rep, bucket, path string, prefix ...string) ([]Metadata, error) {
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}
	if len(prefix) > 0 {
		params.Prefix = aws.String(prefix[0])
	}

	meta, err := getObjects(params, rep, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects for %s: %w", path, err)
	}

	logs.Infof("Retrieved objects for %s", path)

	return meta, nil
}

var GetSegmentedObjects = func(rep, bucket string) ([]Metadata, error) {
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}

	meta, err := getObjects(params, rep, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects for container %s in %s: %w", bucket, ToPrint(rep), err)
	}

	logs.Debugf("Retrieved objects for container %s in %s", bucket, ToPrint(rep))

	return meta, nil
}

func getObjects(params *s3.ListObjectsV2Input, rep, bucket string) ([]Metadata, error) {
	paginator := s3.NewListObjectsV2Paginator(ai.hi.s3Client, params, func(o *s3.ListObjectsV2PaginatorOptions) {
		o.Limit = 10000
	})

	ctx := getContext(rep, false)

	var objects []types.Object
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)

		if err != nil {
			var re *smithyhttp.ResponseError
			if errors.As(err, &re) {
				if re.HTTPStatusCode() == 400 {
					err = fmt.Errorf("bad request: bucket name %q may not be S3 compatible", bucket)
				} else {
					err = re.Err
				}
			}

			return nil, err
		}

		objects = append(objects, output.Contents...)
	}

	meta := make([]Metadata, len(objects))
	for i := range meta {
		meta[i] = Metadata{*objects[i].Key, *objects[i].Size, objects[i].LastModified}
	}

	return meta, nil
}

// DownloadData requests data between range [startDecrypted, endDecrypted).
// As we want to split the data into chunks at consistent locations,
// the requested byte interval may encompass one or two data chunks.
var DownloadData = func(nodes []string, path string, header *string,
	startDecrypted, endDecrypted, oldOffset, fileSize int64,
) ([]byte, error) {
	endDecrypted = min(endDecrypted, fileSize)

	chunkStart := startDecrypted / chunkSize
	chunkEnd := (endDecrypted - 1) / chunkSize

	maxEnd := min((chunkStart+1)*chunkSize, endDecrypted)
	data, err := getDataChunk(nodes, path, header,
		chunkStart, startDecrypted, maxEnd, oldOffset, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get data chunk: %w", err)
	}

	if chunkStart != chunkEnd {
		moreData, err := getDataChunk(nodes, path, header,
			chunkEnd, chunkEnd*chunkSize, endDecrypted, oldOffset, fileSize)
		if err != nil {
			return nil, fmt.Errorf("failed to get second data chunk: %w", err)
		}

		return append(data, moreData...), nil
	}

	return data, nil
}

func getDataChunk(
	nodes []string, path string, header *string,
	chunk, startDecrypted, endDecrypted, oldOffset, fileSize int64,
) ([]byte, error) {
	// start coordinate of chunk
	chByteStart := chunk * chunkSize
	// end coordinate of chunk
	chByteEnd := (chunk + 1) * chunkSize

	// Index offset in chunk
	ofst := startDecrypted - chByteStart
	endofst := endDecrypted - chByteStart

	cacheKey := toCacheKey(nodes, chByteStart)
	chunkData, found := downloadCache.Get(cacheKey)

	if found {
		logs.Debugf("Retrieved file %s from cache, with coordinates [%d, %d)", path, chByteStart+ofst, chByteStart+endofst)

		return chunkData[ofst:endofst], nil
	}

	var startEncrypted, endEncrypted, encryptedBodySize int64
	if header == nil || *header == "" {
		// We end up here when we cannot decrypt the object. Either when trying to fetch the header
		// in filesystem.CheckHeaderExistence() or when object has been encrypted with an unknown key.
		// In these situations there is no point to map the range of bytes to an encrypted range
		// because we read the response body as is.
		startEncrypted = chByteStart
		endEncrypted = chByteEnd
		encryptedBodySize = fileSize
	} else {
		// Convert chunk coordinates to work with encrypted object in storage
		// Chunks are a multiple of BlockSize, so no need to worry about floats
		startEncrypted = chByteStart/BlockSize*CipherBlockSize + oldOffset
		endEncrypted = (chByteEnd+BlockSize-1)/BlockSize*CipherBlockSize + oldOffset

		nBlocks := math.Ceil(float64(fileSize) / float64(BlockSize))
		encryptedBodySize = fileSize + int64(nBlocks)*MacSize + oldOffset
	}
	endEncrypted = min(endEncrypted, encryptedBodySize)

	rep := nodes[1]
	bucket := nodes[3]
	object := strings.Join(nodes[4:], "/")

	ctx := getContext(rep, false)

	resp, err := ai.hi.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", startEncrypted, endEncrypted-1)),
	})
	if err != nil {
		var re *smithyhttp.ResponseError
		if errors.As(err, &re) {
			err = re.Err
		}

		return nil, fmt.Errorf("failed to retrieve object from Allas for %s: %w", path, err)
	}
	defer resp.Body.Close()

	chByteEnd = min(chByteEnd, fileSize)
	buffer := make([]byte, chByteEnd-chByteStart)
	crypt4GHReader := resp.Body.(io.Reader)

	if header != nil && *header != "" { // Encrypted file we can decrypt
		var headerBytes []byte
		headerBytes, err = base64.StdEncoding.DecodeString(*header)
		if err != nil {
			return nil, fmt.Errorf("failed to decode header: %w", err)
		}
		objectReader := io.MultiReader(bytes.NewReader(headerBytes), resp.Body)
		crypt4GHReader, err = streaming.NewCrypt4GHReader(objectReader, ai.vi.privateKey, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to construct reader: %w", err)
		}
	}

	if _, err = io.ReadFull(crypt4GHReader, buffer); err != nil {
		return nil, fmt.Errorf("failed to read file chunk [%d, %d): %w", chByteStart, chByteEnd, err)
	}

	// Do not cache data when retrieving object header
	if header != nil {
		downloadCache.Set(cacheKey, buffer, int64(len(buffer)), time.Minute*60)
		logs.Debugf("File %s stored in cache, with coordinates [%d, %d)", path, chByteStart, chByteEnd)
	}

	return buffer[ofst:endofst], nil
}

// UploadObject uploads object to bucket. Object is uploaded in segments of
// size `segmentSize`. Upload Manager decides if the object is small enough
// to use PutObject, or if multipart upload is necessary.
var UploadObject = func(body io.Reader, rep, bucket, object string, segmentSize int64) error {
	uploader := manager.NewUploader(ai.hi.s3Client, func(u *manager.Uploader) {
		u.PartSize = segmentSize
		u.LeavePartsOnError = false
		u.Concurrency = 1
	})

	ctx := getContext(rep, false)

	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		ContentType: aws.String("application/octet-stream"),
		Bucket:      aws.String(bucket),
		Key:         aws.String(object),
		Body:        body,
	})
	if err != nil {
		var re *smithyhttp.ResponseError
		if errors.As(err, &re) {
			if re.HTTPStatusCode() == 413 {
				err = errors.New("object is too large")
			} else {
				err = re.Err
			}
		}
		if bucket != "" {
			bucket = " to bucket " + bucket
		}

		return fmt.Errorf("failed to upload object %s%s: %w", object, bucket, err)
	}

	return nil
}

// DeleteObject delets object from bucket. Function is necessary for situations where upload
// had to be aborted because something went wrong.
var DeleteObject = func(rep, bucket, object string) error {
	params := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	}

	ctx := getContext(rep, false)

	_, err := ai.hi.s3Client.DeleteObject(ctx, params)
	if err != nil {
		var re *smithyhttp.ResponseError
		if errors.As(err, &re) {
			err = re.Err
		}

		return fmt.Errorf("failed to delete object %s in bucket %s: %w", object, bucket, err)
	}

	return nil
}
