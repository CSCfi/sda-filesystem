# SD Connect Proxy API Reference

The API reflects data in the [SD-Connect](https://research.csc.fi/-/sd-connect) services.

The API is structured into:
- [data](#data-api)
- [metadata](#metadata-api)

## Data API

### GET /data

Data endpoint returns file data (bytes) from a single object.

#### Request URL
```
/data?project={project_name}&container={container_name}&object={object_name}
```

##### Response Headers
If the file was automatically decrypted by the API, the following header is received. This information can be used to calculate the decrypted size of the file (because the initial file size is the encrypted size).
```
X-Decrypted: True
```
If the file is a SLO/DLO segmented file, the following header is received. This information can be used to update the initial file size (which is small for SLOs and zero for DLOs).
```
X-Segmented-Object-Size: 1000000
```

#### Mandatory Headers
Authentication requires the use of two kinds of authorization headers simultaneously:
- `Authorization: Bearer token` is used for SD services (infrastructure authentication),
- `X-Authorization: Basic|Bearer token` is used for Object Stroage (service authentication).

#### X-Authorization for Object Storage
##### Basic
Using `Basic` authorisation with username and password makes the Data API do a hidden token request to Object Storage.
```
X-Authorization: Basic <base64 encoded username:password>
```
##### Bearer
Using `Bearer` authorisation scheme may be faster, as it skips a token request to Object Storage. A token and project ID can be retrieved from the Metadata API's `/token` endpoint.
```
X-Authorization: Bearer <scoped token>
X-Project-ID: <project ID, e.g. abc123>
```

#### Optional Headers
[HTTP Range](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Range) header can be utilised
to retrieve parts of the files.

## Metadata API

Metadata API provides optional token authentication endpoint and endpoints for retrieving information regarding projects, containers and objects.

### GET /token
The token endpoint is an optional authentication endpoint, which can be used to pre-fetch credentials for interfacing with the OpenStack API. Caching tokens may be faster, as using Basic auth in all requests will make the APIs perform authentication and authorisation on every request, but supplying a token will skip those steps.

#### Request
```
/token
```
#### Response
```
{
    "token": "token",
    "projectID": "abc123"
}
```
The `projectID` is important, and required for further requests, where it is placed in the `X-Project-ID` header.

#### Mandatory Headers
Request must be authorised with `Basic` scheme.
```
Authorization: Basic <base64 encoded username:password>
```

### GET /projects
The project endpoint returns a list of projects the user has permissions for (multiple projects have been deprecated, `/projects` now always returns only one project, but the array structure is kept).
#### Request
```
/projects
```
#### Response
```
[
    {
        "name": "project_123",
        "bytes": 1000
    }
]
```

### GET /project/{projectName}/containers
The containers endpoint returns list of containers in a given project.

Optional query params for request, see [OpenStack documentation](https://docs.openstack.org/api-ref/object-store/?expanded=show-container-details-and-list-objects-detail#id18):
- `prefix`
- `delimiter`

#### Request
```
/project/{projectName}/containers
```
#### Response
```
[
    {
        "name": "container-1",
        "bytes": 1000
    },
    {
        "name": "container-2",
        "bytes": 1000
    }
]
```
#### Mandatory Headers
Request must be authorised with `Basic` or `Bearer` scheme.

##### Basic
Using `Basic` authorisation with username and password makes the Data API do a hidden token request to Object Storage.
```
X-Authorization: Basic <base64 encoded username:password>
```
##### Bearer
Using `Bearer` authorisation scheme may be faster, as it skips a token request to Object Storage. A token and project ID can be retrieved from the Metadata API's `/token` endpoint.
```
X-Authorization: Bearer <scoped token>
X-Project-ID: <project ID, e.g. abc123>
```

### GET /project/{projectName}/container/{containerName}/objects
The objects endpoint returns list of objects in a given project container.
#### Request
```
/project/{projectName}/container/{containerName}/objects
```
#### Response
```json
[
    {
        "name": "file.txt",
        "bytes": 100
    },
    {
        "name": "image.jpg",
        "bytes": 100
    }
]
```
#### Mandatory Headers
Request must be authorised with `Basic` or `Bearer` scheme.

##### Basic
Using `Basic` authorisation with username and password makes the Data API do a hidden token request to Object Storage.
```
X-Authorization: Basic <base64 encoded username:password>
```
##### Bearer
Using `Bearer` authorisation scheme may be faster, as it skips a token request to Object Storage. A token and project ID can be retrieved from the Metadata API's `/token` endpoint.
```
X-Authorization: Bearer <scoped token>
X-Project-ID: <project ID, e.g. abc123>
```
