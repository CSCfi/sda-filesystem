# SD Connect FUSE
This desktop software converses with the [sd-connect-metadata-api](https://gitlab.ci.csc.fi/sds-dev/sd-connect-metadata-api) and [sd-connect-data-api](https://gitlab.ci.csc.fi/sds-dev/sd-connect-data-api) to download files.

## Dependencies
See [cgofuse](https://github.com/billziss-gh/cgofuse#how-to-build) for dependencies on different operating systems.

Install [Qt for Go](https://github.com/therecipe/qt/wiki/Installation). Regardless of the operating system, there are multiple ways of installing this package. For me installation worked when `GO111MODULE=on`. There seems to be [several ways](https://github.com/therecipe/qt/wiki/Available-Tools) of running a go module which uses qt. Below is the way I am currently running this program. Need to look into this further.

## Run
Set environment variables `SD_CONNECT_METADATA_API`, `SD_CONNECT_DATA_API` and `SD_CONNECT_CERTS` before running program.
```
export SD_CONNECT_METADATA_API=https://connect-metadata-api-test.sd.csc.fi
export SD_CONNECT_DATA_API=https://connect-data-api-test.sd.csc.fi
export SD_CONNECT_CERTS=cert.pem

qtdeploy build desktop
./deploy/darwin/sd-connect-fuse.app/Contents/MacOS/sd-connect-fuse  // Path slightly different for other OSs
