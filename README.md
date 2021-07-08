# SD Connect FUSE
This desktop software converses with the [sd-connect-metadata-api](https://gitlab.ci.csc.fi/sds-dev/sd-connect-metadata-api) and [sd-connect-data-api](https://gitlab.ci.csc.fi/sds-dev/sd-connect-data-api) to download files.

## Dependencies
See [cgofuse](https://github.com/billziss-gh/cgofuse#how-to-build) for dependencies on different operating systems.

## Run
Set environment variables `SD_CONNECT_METADATA_API`, `SD_CONNECT_DATA_API` and `SD_CONNECT_CERTS` before running program.
```
export SD_CONNECT_METADATA_API=https://connect-metadata-api-test.sd.csc.fi
export SD_CONNECT_DATA_API=https://connect-data-api-test.sd.csc.fi
export SD_CONNECT_CERTS=cert.pem

go run cmd/main.go
