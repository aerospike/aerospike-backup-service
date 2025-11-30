# RestoreApi

All URIs are relative to _http://localhost:8080_

| Method                                                           | HTTP request                                          | Description                                                          |
| ---------------------------------------------------------------- | ----------------------------------------------------- | -------------------------------------------------------------------- |
| [**cancelRestore**](RestoreApi.md#cancelrestore)                 | **POST** /v1/restore/cancel/{jobId}                   | Cancel a running restore operation.                                  |
| [**restoreFull**](RestoreApi.md#restorefull)                     | **POST** /v1/restore/full                             | Trigger an asynchronous full restore operation.                      |
| [**restoreIncremental**](RestoreApi.md#restoreincremental)       | **POST** /v1/restore/incremental                      | Trigger an asynchronous incremental restore operation.               |
| [**restoreStatus**](RestoreApi.md#restorestatus)                 | **GET** /v1/restore/status/{jobId}                    | Retrieve status for a restore job.                                   |
| [**restoreTimestamp**](RestoreApi.md#restoretimestamp)           | **POST** /v1/restore/timestamp                        | Trigger an asynchronous restore operation to specific point in time. |
| [**retrieveConfiguration**](RestoreApi.md#retrieveconfiguration) | **GET** /v1/retrieve/configuration/{name}/{timestamp} | Retrieve Aerospike cluster configuration backup                      |
| [**retrieveRestoreJobs**](RestoreApi.md#retrieverestorejobs)     | **GET** /v1/restore/jobs                              | Retrieve restore jobs.                                               |

## cancelRestore

> string cancelRestore(jobId)

Cancel a running restore operation.

### Example

```ts
import { Configuration, RestoreApi } from "";
import type { CancelRestoreRequest } from "";

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RestoreApi();

  const body = {
    // number | Restore job ID
    jobId: 789,
  } satisfies CancelRestoreRequest;

  try {
    const data = await api.cancelRestore(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name      | Type     | Description    | Notes                     |
| --------- | -------- | -------------- | ------------------------- |
| **jobId** | `number` | Restore job ID | [Defaults to `undefined`] |

### Return type

**string**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `*/*`

### HTTP response details

| Status code | Description                       | Response headers |
| ----------- | --------------------------------- | ---------------- |
| **202**     | Restore job canceled successfully | -                |
| **400**     | Invalid job ID                    | -                |
| **404**     | Job not found                     | -                |
| **500**     | Internal server error             | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## restoreFull

> number restoreFull(dtoRestoreRequest)

Trigger an asynchronous full restore operation.

### Example

```ts
import {
  Configuration,
  RestoreApi,
} from '';
import type { RestoreFullRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RestoreApi();

  const body = {
    // DtoRestoreRequest | Restore request details
    dtoRestoreRequest: ...,
  } satisfies RestoreFullRequest;

  try {
    const data = await api.restoreFull(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name                  | Type                                      | Description             | Notes |
| --------------------- | ----------------------------------------- | ----------------------- | ----- |
| **dtoRestoreRequest** | [DtoRestoreRequest](DtoRestoreRequest.md) | Restore request details |       |

### Return type

**number**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`

### HTTP response details

| Status code | Description              | Response headers |
| ----------- | ------------------------ | ---------------- |
| **202**     | Restore operation job id | -                |
| **400**     | Bad Request              | -                |
| **405**     | Method Not Allowed       | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## restoreIncremental

> number restoreIncremental(dtoRestoreRequest)

Trigger an asynchronous incremental restore operation.

### Example

```ts
import {
  Configuration,
  RestoreApi,
} from '';
import type { RestoreIncrementalRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RestoreApi();

  const body = {
    // DtoRestoreRequest | Restore request details
    dtoRestoreRequest: ...,
  } satisfies RestoreIncrementalRequest;

  try {
    const data = await api.restoreIncremental(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name                  | Type                                      | Description             | Notes |
| --------------------- | ----------------------------------------- | ----------------------- | ----- |
| **dtoRestoreRequest** | [DtoRestoreRequest](DtoRestoreRequest.md) | Restore request details |       |

### Return type

**number**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`

### HTTP response details

| Status code | Description              | Response headers |
| ----------- | ------------------------ | ---------------- |
| **202**     | Restore operation job id | -                |
| **400**     | Bad Request              | -                |
| **405**     | Method Not Allowed       | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## restoreStatus

> DtoRestoreJobStatus restoreStatus(jobId)

Retrieve status for a restore job.

### Example

```ts
import { Configuration, RestoreApi } from "";
import type { RestoreStatusRequest } from "";

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RestoreApi();

  const body = {
    // number | Job ID to retrieve the status
    jobId: 789,
  } satisfies RestoreStatusRequest;

  try {
    const data = await api.restoreStatus(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name      | Type     | Description                   | Notes                     |
| --------- | -------- | ----------------------------- | ------------------------- |
| **jobId** | `number` | Job ID to retrieve the status | [Defaults to `undefined`] |

### Return type

[**DtoRestoreJobStatus**](DtoRestoreJobStatus.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`

### HTTP response details

| Status code | Description                | Response headers |
| ----------- | -------------------------- | ---------------- |
| **200**     | Restore job status details | -                |
| **400**     | Bad Request                | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## restoreTimestamp

> number restoreTimestamp(dtoRestoreTimestampRequest)

Trigger an asynchronous restore operation to specific point in time.

Restores backup from the given point in time.

### Example

```ts
import {
  Configuration,
  RestoreApi,
} from '';
import type { RestoreTimestampRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RestoreApi();

  const body = {
    // DtoRestoreTimestampRequest | Restore request details
    dtoRestoreTimestampRequest: ...,
  } satisfies RestoreTimestampRequest;

  try {
    const data = await api.restoreTimestamp(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name                           | Type                                                        | Description             | Notes |
| ------------------------------ | ----------------------------------------------------------- | ----------------------- | ----- |
| **dtoRestoreTimestampRequest** | [DtoRestoreTimestampRequest](DtoRestoreTimestampRequest.md) | Restore request details |       |

### Return type

**number**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`

### HTTP response details

| Status code | Description              | Response headers |
| ----------- | ------------------------ | ---------------- |
| **202**     | Restore operation job id | -                |
| **400**     | Bad Request              | -                |
| **405**     | Method Not Allowed       | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## retrieveConfiguration

> Blob retrieveConfiguration(name, timestamp)

Retrieve Aerospike cluster configuration backup

### Example

```ts
import { Configuration, RestoreApi } from "";
import type { RetrieveConfigurationRequest } from "";

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RestoreApi();

  const body = {
    // string | Backup routine name
    name: name_example,
    // number | Backup timestamp
    timestamp: 789,
  } satisfies RetrieveConfigurationRequest;

  try {
    const data = await api.retrieveConfiguration(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name          | Type     | Description         | Notes                     |
| ------------- | -------- | ------------------- | ------------------------- |
| **name**      | `string` | Backup routine name | [Defaults to `undefined`] |
| **timestamp** | `number` | Backup timestamp    | [Defaults to `undefined`] |

### Return type

**Blob**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/zip`

### HTTP response details

| Status code | Description        | Response headers |
| ----------- | ------------------ | ---------------- |
| **200**     | OK                 | -                |
| **400**     | Bad Request        | -                |
| **405**     | Method Not Allowed | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## retrieveRestoreJobs

> { [key: string]: DtoRestoreJobStatus; } retrieveRestoreJobs(from, to, status)

Retrieve restore jobs.

### Example

```ts
import { Configuration, RestoreApi } from "";
import type { RetrieveRestoreJobsRequest } from "";

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new RestoreApi();

  const body = {
    // number | Lower bound timestamp filter (optional)
    from: 789,
    // number | Upper bound timestamp filter (optional)
    to: 789,
    // string | Comma-separated status filter (Running,Done,Failed,Cancelled). Use ! prefix for exclude filter (e.g., !Failed,Cancelled) (optional)
    status: status_example,
  } satisfies RetrieveRestoreJobsRequest;

  try {
    const data = await api.retrieveRestoreJobs(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name       | Type     | Description                                                                                                              | Notes                                |
| ---------- | -------- | ------------------------------------------------------------------------------------------------------------------------ | ------------------------------------ |
| **from**   | `number` | Lower bound timestamp filter                                                                                             | [Optional] [Defaults to `undefined`] |
| **to**     | `number` | Upper bound timestamp filter                                                                                             | [Optional] [Defaults to `undefined`] |
| **status** | `string` | Comma-separated status filter (Running,Done,Failed,Cancelled). Use ! prefix for exclude filter (e.g., !Failed,Cancelled) | [Optional] [Defaults to `undefined`] |

### Return type

[**{ [key: string]: DtoRestoreJobStatus; }**](DtoRestoreJobStatus.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`

### HTTP response details

| Status code | Description           | Response headers |
| ----------- | --------------------- | ---------------- |
| **200**     | Restore jobs          | -                |
| **400**     | Bad Request           | -                |
| **500**     | Internal Server Error | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
