# SD Apply/SD Submit API Reference

The API reflects data in the [FEGA](https://research.csc.fi/-/fega) and [SD Apply/SD Submit](https://research.csc.fi/sensitive-data-services-for-research) services.

The API implementation is in [sda-download](https://github.com/neicnordic/sda-download) which is a Go implementation of the NeIC [Data Out API](https://neic-sda.readthedocs.io/en/latest/dataout.html#rest-api-endpoints).

All endpoints require an `Authorization` header with an access token in the `Bearer` scheme.

```
Authorization: Bearer <token>
```

The API is structured into:
- metadata:
    - [datasets](#datasets)
    - [files](#files)
- file download [files](#file-download)

### Authenticated Session
The client can establish a session to skip time-costly visa validations for further requests. Session is based on the `sda_session_key` cookie returned by the server, which should be returned in later requests. This is done automatically with a cookie jar.

## Datasets

The `/metadata/datasets` endpoint is used to display the list of datasets the given token is authorised to access, that are present in the archive.

### Request
```
GET /metadata/datasets
```
### Response
```
[
    "dataset_1",
    "dataset_2"
]
```
## Files

### Request

Files contained by a dataset are listed using the `datasetName` from `/metadata/datasets`.
```
GET /metadata/datasets/{datasetName}/files
```

#### Optional Scheme Parameter
If a dataset name is in URI format, e.g. `https://repository.org/dataset`, the scheme `https` can be split with `://` and attached to a `scheme` query parameter.
```
GET /metadata/datasets/repository.org/dataset/files?scheme=https
```

### Response
```
[
    {
        "fileId": "urn:file:1",
        "datasetId": "dataset_1",
        "displayFileName": "file_1.txt.c4gh",
        "fileName": "hash",
        "fileSize": 60,
        "decryptedFileSize": 32,
        "decryptedFileChecksum": "hash",
        "decryptedFileChecksumType": "SHA256",
        "fileStatus": "READY"
    },
    {
        "fileId": "urn:file:2",
        "datasetId": "dataset_1",
        "displayFileName": "file_2.txt.c4gh",
        "fileName": "hash",
        "fileSize": 60,
        "decryptedFileSize": 32,
        "decryptedFileChecksum": "hash",
        "decryptedFileChecksumType": "SHA256",
        "fileStatus": "READY"
    },
]
```
## File Download

File data is downloaded using the `fileId` from `/metadata/datasets/{datasetName}/files`.

### Request
```
GET /files/{fileId}
```

### Response

Response is given as byte stream `application/octet-stream`
```
hello
```

### Optional Query Parameters

Parts of a file can be requested with specific byte ranges using `startCoordinate` and `endCoordinate` query parameters, e.g.:
```
?startCoordinate=0&endCoordinate=100
```
