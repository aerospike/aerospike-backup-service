# BackupApi

All URIs are relative to *http://localhost:8080*

| Method                                                                              | HTTP request                             | Description                                   |
|-------------------------------------------------------------------------------------|------------------------------------------|-----------------------------------------------|
| [**cancelCurrentBackup**](BackupApi.md#cancelcurrentbackup)                         | **POST** /v1/backups/cancel/{name}       | Cancel current backup.                        |
| [**getCurrentBackup**](BackupApi.md#getcurrentbackup)                               | **GET** /v1/backups/currentBackup/{name} | Get current backup statistics.                |
| [**getFullBackups**](BackupApi.md#getfullbackups)                                   | **GET** /v1/backups/full                 | Get available full backups.                   |
| [**getFullBackupsForRoutine**](BackupApi.md#getfullbackupsforroutine)               | **GET** /v1/backups/full/{name}          | Get available full backups for routine.       |
| [**getIncrementalBackups**](BackupApi.md#getincrementalbackups)                     | **GET** /v1/backups/incremental          | Get available incremental backups.            |
| [**getIncrementalBackupsForRoutine**](BackupApi.md#getincrementalbackupsforroutine) | **GET** /v1/backups/incremental/{name}   | Get incremental backups for routine.          |
| [**scheduleFullBackup**](BackupApi.md#schedulefullbackup)                           | **POST** /v1/backups/schedule/{name}     | Schedule a full backup once per routine name. |

## cancelCurrentBackup

> cancelCurrentBackup(name)

Cancel current backup.

### Example

```ts
import {
  Configuration,
  BackupApi,
} from '';
import type { CancelCurrentBackupRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BackupApi();

  const body = {
    // string | Backup routine name
    name: name_example,
  } satisfies CancelCurrentBackupRequest;

  try {
    const data = await api.cancelCurrentBackup(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **name** | `string` | Backup routine name | [Defaults to `undefined`] |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `*/*`


### HTTP response details

| Status code | Description           | Response headers |
|-------------|-----------------------|------------------|
| **202**     | Accepted              | -                |
| **404**     | Not Found             | -                |
| **500**     | Internal Server Error | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## getCurrentBackup

> DtoRoutineState getCurrentBackup(name)

Get current backup statistics.

### Example

```ts
import {
  Configuration,
  BackupApi,
} from '';
import type { GetCurrentBackupRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BackupApi();

  const body = {
    // string | Backup routine name
    name: name_example,
  } satisfies GetCurrentBackupRequest;

  try {
    const data = await api.getCurrentBackup(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **name** | `string` | Backup routine name | [Defaults to `undefined`] |

### Return type

[**DtoRoutineState**](DtoRoutineState.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details

| Status code | Description               | Response headers |
|-------------|---------------------------|------------------|
| **200**     | Current backup statistics | -                |
| **400**     | Bad Request               | -                |
| **404**     | Not Found                 | -                |
| **500**     | Internal Server Error     | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## getFullBackups

> { [key: string]: Array&lt;DtoBackupDetails&gt;; } getFullBackups(from, to)

Get available full backups.

### Example

```ts
import {
  Configuration,
  BackupApi,
} from '';
import type { GetFullBackupsRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BackupApi();

  const body = {
    // number | Lower bound timestamp filter (optional)
    from: 789,
    // number | Upper bound timestamp filter (optional)
    to: 789,
  } satisfies GetFullBackupsRequest;

  try {
    const data = await api.getFullBackups(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name     | Type     | Description                  | Notes                                |
|----------|----------|------------------------------|--------------------------------------|
| **from** | `number` | Lower bound timestamp filter | [Optional] [Defaults to `undefined`] |
| **to**   | `number` | Upper bound timestamp filter | [Optional] [Defaults to `undefined`] |

### Return type

**{ [key: string]: Array<DtoBackupDetails>; }**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details

| Status code | Description             | Response headers |
|-------------|-------------------------|------------------|
| **200**     | Full backups by routine | -                |
| **400**     | Bad Request             | -                |
| **500**     | Internal Server Error   | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## getFullBackupsForRoutine

> Array&lt;DtoBackupDetails&gt; getFullBackupsForRoutine(name, from, to)

Get available full backups for routine.

### Example

```ts
import {
  Configuration,
  BackupApi,
} from '';
import type { GetFullBackupsForRoutineRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BackupApi();

  const body = {
    // string | Backup routine name
    name: name_example,
    // number | Lower bound timestamp filter (optional)
    from: 789,
    // number | Upper bound timestamp filter (optional)
    to: 789,
  } satisfies GetFullBackupsForRoutineRequest;

  try {
    const data = await api.getFullBackupsForRoutine(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name     | Type     | Description                  | Notes                                |
|----------|----------|------------------------------|--------------------------------------|
| **name** | `string` | Backup routine name          | [Defaults to `undefined`]            |
| **from** | `number` | Lower bound timestamp filter | [Optional] [Defaults to `undefined`] |
| **to**   | `number` | Upper bound timestamp filter | [Optional] [Defaults to `undefined`] |

### Return type

[**Array&lt;DtoBackupDetails&gt;**](DtoBackupDetails.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details

| Status code | Description              | Response headers |
|-------------|--------------------------|------------------|
| **200**     | Full backups for routine | -                |
| **400**     | Bad Request              | -                |
| **500**     | Internal Server Error    | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## getIncrementalBackups

> { [key: string]: Array&lt;DtoBackupDetails&gt;; } getIncrementalBackups(from, to)

Get available incremental backups.

### Example

```ts
import {
  Configuration,
  BackupApi,
} from '';
import type { GetIncrementalBackupsRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BackupApi();

  const body = {
    // number | Lower bound timestamp filter (optional)
    from: 789,
    // number | Upper bound timestamp filter (optional)
    to: 789,
  } satisfies GetIncrementalBackupsRequest;

  try {
    const data = await api.getIncrementalBackups(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name     | Type     | Description                  | Notes                                |
|----------|----------|------------------------------|--------------------------------------|
| **from** | `number` | Lower bound timestamp filter | [Optional] [Defaults to `undefined`] |
| **to**   | `number` | Upper bound timestamp filter | [Optional] [Defaults to `undefined`] |

### Return type

**{ [key: string]: Array<DtoBackupDetails>; }**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`

### HTTP response details

| Status code | Description                    | Response headers |
|-------------|--------------------------------|------------------|
| **200**     | Incremental backups by routine | -                |
| **400**     | Bad Request                    | -                |
| **500**     | Internal Server Error          | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## getIncrementalBackupsForRoutine

> Array&lt;DtoBackupDetails&gt; getIncrementalBackupsForRoutine(name, from, to)

Get incremental backups for routine.

### Example

```ts
import {
  Configuration,
  BackupApi,
} from '';
import type { GetIncrementalBackupsForRoutineRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BackupApi();

  const body = {
    // string | Backup routine name
    name: name_example,
    // number | Lower bound timestamp filter (optional)
    from: 789,
    // number | Upper bound timestamp filter (optional)
    to: 789,
  } satisfies GetIncrementalBackupsForRoutineRequest;

  try {
    const data = await api.getIncrementalBackupsForRoutine(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name     | Type     | Description                  | Notes                                |
|----------|----------|------------------------------|--------------------------------------|
| **name** | `string` | Backup routine name          | [Defaults to `undefined`]            |
| **from** | `number` | Lower bound timestamp filter | [Optional] [Defaults to `undefined`] |
| **to**   | `number` | Upper bound timestamp filter | [Optional] [Defaults to `undefined`] |

### Return type

[**Array&lt;DtoBackupDetails&gt;**](DtoBackupDetails.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`

### HTTP response details

| Status code | Description                     | Response headers |
|-------------|---------------------------------|------------------|
| **200**     | Incremental backups for routine | -                |
| **400**     | Bad Request                     | -                |
| **500**     | Internal Server Error           | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## scheduleFullBackup

> scheduleFullBackup(name, delay)

Schedule a full backup once per routine name.

### Example

```ts
import {
  Configuration,
  BackupApi,
} from '';
import type { ScheduleFullBackupRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new BackupApi();

  const body = {
    // string | Backup routine name
    name: name_example,
    // number | Delay interval in milliseconds (optional)
    delay: 56,
  } satisfies ScheduleFullBackupRequest;

  try {
    const data = await api.scheduleFullBackup(body);
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

| Name | Type | Description  | Notes |
|------------- | ------------- | ------------- | -------------|
| **name** | `string` | Backup routine name | [Defaults to `undefined`] |
| **delay** | `number` | Delay interval in milliseconds | [Optional] [Defaults to `undefined`] |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `*/*`

### HTTP response details

| Status code | Description           | Response headers |
|-------------|-----------------------|------------------|
| **202**     | Accepted              | -                |
| **400**     | Bad Request           | -                |
| **404**     | Not Found             | -                |
| **500**     | Internal Server Error | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

