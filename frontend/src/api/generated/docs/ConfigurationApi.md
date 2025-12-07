# ConfigurationApi

All URIs are relative to *http://localhost:8080*

| Method | HTTP request | Description |
|------------- | ------------- | -------------|
| [**addCluster**](ConfigurationApi.md#addcluster) | **POST** /v1/config/clusters/{name} | Adds an Aerospike cluster to the config. |
| [**addPolicy**](ConfigurationApi.md#addpolicy) | **POST** /v1/config/policies/{name} | Adds a policy to the config. |
| [**addRoutine**](ConfigurationApi.md#addroutine) | **POST** /v1/config/routines/{name} | Adds a backup routine to the config. |
| [**addStorage**](ConfigurationApi.md#addstorage) | **POST** /v1/config/storage/{name} | Adds a storage to the config. |
| [**applyConfig**](ConfigurationApi.md#applyconfig) | **POST** /v1/config/apply | Reloads the configuration from the config file. |
| [**checkClusterConnectivity**](ConfigurationApi.md#checkclusterconnectivity) | **POST** /v1/config/clusters/check-connectivity | Checks connectivity to an Aerospike cluster. |
| [**checkSecretAgentConnectivity**](ConfigurationApi.md#checksecretagentconnectivity) | **POST** /v1/config/secret-agents/check-connectivity | Checks connectivity to a Secret Agent. |
| [**checkStorageConnectivity**](ConfigurationApi.md#checkstorageconnectivity) | **POST** /v1/config/storage/check-connectivity | Checks connectivity to a backup storage. |
| [**deleteCluster**](ConfigurationApi.md#deletecluster) | **DELETE** /v1/config/clusters/{name} | Deletes a cluster from the configuration by name. |
| [**deletePolicy**](ConfigurationApi.md#deletepolicy) | **DELETE** /v1/config/policies/{name} | Deletes a policy from the configuration by name. |
| [**deleteRoutine**](ConfigurationApi.md#deleteroutine) | **DELETE** /v1/config/routines/{name} | Deletes a backup routine from the configuration by name. |
| [**deleteStorage**](ConfigurationApi.md#deletestorage) | **DELETE** /v1/config/storage/{name} | Deletes a storage from the configuration by name. |
| [**disableRoutine**](ConfigurationApi.md#disableroutine) | **PUT** /v1/config/routines/{name}/disable | Disable a backup routine. |
| [**enableRoutine**](ConfigurationApi.md#enableroutine) | **PUT** /v1/config/routines/{name}/enable | Enable a backup routine. |
| [**readAllClusters**](ConfigurationApi.md#readallclusters) | **GET** /v1/config/clusters | Reads all Aerospike clusters from the configuration. |
| [**readAllStorage**](ConfigurationApi.md#readallstorage) | **GET** /v1/config/storage | Reads all storage from the configuration. |
| [**readCluster**](ConfigurationApi.md#readcluster) | **GET** /v1/config/clusters/{name} | Reads a specific Aerospike cluster from the configuration given its name. |
| [**readConfig**](ConfigurationApi.md#readconfig) | **GET** /v1/config | Returns the configuration for the service. |
| [**readPolicies**](ConfigurationApi.md#readpolicies) | **GET** /v1/config/policies | Reads all policies from the configuration. |
| [**readPolicy**](ConfigurationApi.md#readpolicy) | **GET** /v1/config/policies/{name} | Reads a backup policy from the configuration given its name. |
| [**readRoutine**](ConfigurationApi.md#readroutine) | **GET** /v1/config/routines/{name} | Reads a specific routine from the configuration given its name. |
| [**readRoutines**](ConfigurationApi.md#readroutines) | **GET** /v1/config/routines | Reads all routines from the configuration. |
| [**readStorage**](ConfigurationApi.md#readstorage) | **GET** /v1/config/storage/{name} | Reads a specific storage from the configuration given its name. |
| [**updateCluster**](ConfigurationApi.md#updatecluster) | **PUT** /v1/config/clusters/{name} | Updates an existing Aerospike cluster in the configuration. |
| [**updateConfig**](ConfigurationApi.md#updateconfig) | **PUT** /v1/config | Updates the configuration for the service. |
| [**updatePolicy**](ConfigurationApi.md#updatepolicy) | **PUT** /v1/config/policies/{name} | Updates an existing policy in the configuration. |
| [**updateRoutine**](ConfigurationApi.md#updateroutine) | **PUT** /v1/config/routines/{name} | Updates an existing routine in the configuration. |
| [**updateStorage**](ConfigurationApi.md#updatestorage) | **PUT** /v1/config/storage/{name} | Updates an existing storage in the configuration. |



## addCluster

> addCluster(name, dtoAerospikeCluster)

Adds an Aerospike cluster to the config.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { AddClusterRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Aerospike cluster name
    name: name_example,
    // DtoAerospikeCluster | Aerospike cluster details
    dtoAerospikeCluster: ...,
  } satisfies AddClusterRequest;

  try {
    const data = await api.addCluster(body);
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
| **name** | `string` | Aerospike cluster name | [Defaults to `undefined`] |
| **dtoAerospikeCluster** | [DtoAerospikeCluster](DtoAerospikeCluster.md) | Aerospike cluster details | |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **201** | Created |  -  |
| **400** | Bad Request |  -  |
| **500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## addPolicy

> addPolicy(name, dtoBackupPolicy)

Adds a policy to the config.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { AddPolicyRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup policy name
    name: name_example,
    // DtoBackupPolicy | Backup policy details
    dtoBackupPolicy: ...,
  } satisfies AddPolicyRequest;

  try {
    const data = await api.addPolicy(body);
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
| **name** | `string` | Backup policy name | [Defaults to `undefined`] |
| **dtoBackupPolicy** | [DtoBackupPolicy](DtoBackupPolicy.md) | Backup policy details | |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **201** | Created |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## addRoutine

> addRoutine(name, dtoBackupRoutine)

Adds a backup routine to the config.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { AddRoutineRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup routine name
    name: name_example,
    // DtoBackupRoutine | Backup routine details
    dtoBackupRoutine: ...,
  } satisfies AddRoutineRequest;

  try {
    const data = await api.addRoutine(body);
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
| **dtoBackupRoutine** | [DtoBackupRoutine](DtoBackupRoutine.md) | Backup routine details | |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **201** | Created |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## addStorage

> addStorage(name, dtoStorage)

Adds a storage to the config.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { AddStorageRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup storage name
    name: name_example,
    // DtoStorage | Backup storage details
    dtoStorage: ...,
  } satisfies AddStorageRequest;

  try {
    const data = await api.addStorage(body);
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
| **name** | `string` | Backup storage name | [Defaults to `undefined`] |
| **dtoStorage** | [DtoStorage](DtoStorage.md) | Backup storage details | |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **201** | Created |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## applyConfig

> applyConfig()

Reloads the configuration from the config file.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { ApplyConfigRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  try {
    const data = await api.applyConfig();
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

This endpoint does not need any parameter.

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## checkClusterConnectivity

> { [key: string]: Array&lt;string&gt;; } checkClusterConnectivity(dtoAerospikeCluster)

Checks connectivity to an Aerospike cluster.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { CheckClusterConnectivityRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // DtoAerospikeCluster | Aerospike cluster details
    dtoAerospikeCluster: ...,
  } satisfies CheckClusterConnectivityRequest;

  try {
    const data = await api.checkClusterConnectivity(body);
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
| **dtoAerospikeCluster** | [DtoAerospikeCluster](DtoAerospikeCluster.md) | Aerospike cluster details | |

### Return type

**{ [key: string]: Array<string>; }**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |
| **500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## checkSecretAgentConnectivity

> string checkSecretAgentConnectivity(dtoSecretAgent)

Checks connectivity to a Secret Agent.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { CheckSecretAgentConnectivityRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // DtoSecretAgent | Secret Agent details
    dtoSecretAgent: ...,
  } satisfies CheckSecretAgentConnectivityRequest;

  try {
    const data = await api.checkSecretAgentConnectivity(body);
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
| **dtoSecretAgent** | [DtoSecretAgent](DtoSecretAgent.md) | Secret Agent details | |

### Return type

**string**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Connection successful |  -  |
| **400** | Bad Request |  -  |
| **500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## checkStorageConnectivity

> string checkStorageConnectivity(dtoStorage)

Checks connectivity to a backup storage.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { CheckStorageConnectivityRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // DtoStorage | Backup storage details
    dtoStorage: ...,
  } satisfies CheckStorageConnectivityRequest;

  try {
    const data = await api.checkStorageConnectivity(body);
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
| **dtoStorage** | [DtoStorage](DtoStorage.md) | Backup storage details | |

### Return type

**string**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | Connection successful |  -  |
| **400** | Bad Request |  -  |
| **500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## deleteCluster

> deleteCluster(name)

Deletes a cluster from the configuration by name.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { DeleteClusterRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Aerospike cluster name
    name: name_example,
  } satisfies DeleteClusterRequest;

  try {
    const data = await api.deleteCluster(body);
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
| **name** | `string` | Aerospike cluster name | [Defaults to `undefined`] |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **204** | No Content |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## deletePolicy

> deletePolicy(name)

Deletes a policy from the configuration by name.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { DeletePolicyRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup policy name
    name: name_example,
  } satisfies DeletePolicyRequest;

  try {
    const data = await api.deletePolicy(body);
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
| **name** | `string` | Backup policy name | [Defaults to `undefined`] |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **204** | No Content |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## deleteRoutine

> deleteRoutine(name)

Deletes a backup routine from the configuration by name.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { DeleteRoutineRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup routine name
    name: name_example,
  } satisfies DeleteRoutineRequest;

  try {
    const data = await api.deleteRoutine(body);
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
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **204** | No Content |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## deleteStorage

> deleteStorage(name)

Deletes a storage from the configuration by name.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { DeleteStorageRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup storage name
    name: name_example,
  } satisfies DeleteStorageRequest;

  try {
    const data = await api.deleteStorage(body);
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
| **name** | `string` | Backup storage name | [Defaults to `undefined`] |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **204** | No Content |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## disableRoutine

> disableRoutine(name)

Disable a backup routine.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { DisableRoutineRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | The name of the backup routine.
    name: name_example,
  } satisfies DisableRoutineRequest;

  try {
    const data = await api.disableRoutine(body);
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
| **name** | `string` | The name of the backup routine. | [Defaults to `undefined`] |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **204** | Routine successfully disabled. |  -  |
| **404** | Not Found |  -  |
| **500** | Unexpected error occurred. |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## enableRoutine

> enableRoutine(name)

Enable a backup routine.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { EnableRoutineRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup routine name
    name: name_example,
  } satisfies EnableRoutineRequest;

  try {
    const data = await api.enableRoutine(body);
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
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **204** | Routine successfully enabled. |  -  |
| **404** | Not Found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## readAllClusters

> { [key: string]: DtoAerospikeCluster; } readAllClusters()

Reads all Aerospike clusters from the configuration.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { ReadAllClustersRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  try {
    const data = await api.readAllClusters();
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

This endpoint does not need any parameter.

### Return type

[**{ [key: string]: DtoAerospikeCluster; }**](DtoAerospikeCluster.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## readAllStorage

> { [key: string]: DtoStorage; } readAllStorage()

Reads all storage from the configuration.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { ReadAllStorageRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  try {
    const data = await api.readAllStorage();
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

This endpoint does not need any parameter.

### Return type

[**{ [key: string]: DtoStorage; }**](DtoStorage.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## readCluster

> DtoAerospikeCluster readCluster(name)

Reads a specific Aerospike cluster from the configuration given its name.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { ReadClusterRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Aerospike cluster name
    name: name_example,
  } satisfies ReadClusterRequest;

  try {
    const data = await api.readCluster(body);
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
| **name** | `string` | Aerospike cluster name | [Defaults to `undefined`] |

### Return type

[**DtoAerospikeCluster**](DtoAerospikeCluster.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |
| **404** | The specified cluster could not be found |  -  |
| **500** | The specified cluster could not be found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## readConfig

> DtoConfig readConfig()

Returns the configuration for the service.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { ReadConfigRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  try {
    const data = await api.readConfig();
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

This endpoint does not need any parameter.

### Return type

[**DtoConfig**](DtoConfig.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## readPolicies

> { [key: string]: DtoBackupPolicy; } readPolicies()

Reads all policies from the configuration.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { ReadPoliciesRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  try {
    const data = await api.readPolicies();
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

This endpoint does not need any parameter.

### Return type

[**{ [key: string]: DtoBackupPolicy; }**](DtoBackupPolicy.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## readPolicy

> DtoBackupPolicy readPolicy(name)

Reads a backup policy from the configuration given its name.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { ReadPolicyRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup policy name
    name: name_example,
  } satisfies ReadPolicyRequest;

  try {
    const data = await api.readPolicy(body);
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
| **name** | `string` | Backup policy name | [Defaults to `undefined`] |

### Return type

[**DtoBackupPolicy**](DtoBackupPolicy.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |
| **404** | The specified policy could not be found |  -  |
| **500** | The specified policy could not be found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## readRoutine

> DtoBackupRoutine readRoutine(name)

Reads a specific routine from the configuration given its name.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { ReadRoutineRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup routine name
    name: name_example,
  } satisfies ReadRoutineRequest;

  try {
    const data = await api.readRoutine(body);
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

[**DtoBackupRoutine**](DtoBackupRoutine.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |
| **404** | The specified routine could not be found |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## readRoutines

> { [key: string]: DtoBackupRoutine; } readRoutines()

Reads all routines from the configuration.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { ReadRoutinesRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  try {
    const data = await api.readRoutines();
    console.log(data);
  } catch (error) {
    console.error(error);
  }
}

// Run the test
example().catch(console.error);
```

### Parameters

This endpoint does not need any parameter.

### Return type

[**{ [key: string]: DtoBackupRoutine; }**](DtoBackupRoutine.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## readStorage

> DtoStorage readStorage(name)

Reads a specific storage from the configuration given its name.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { ReadStorageRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup storage name
    name: name_example,
  } satisfies ReadStorageRequest;

  try {
    const data = await api.readStorage(body);
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
| **name** | `string` | Backup storage name | [Defaults to `undefined`] |

### Return type

[**DtoStorage**](DtoStorage.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `application/json`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |
| **404** | The specified storage could not be found |  -  |
| **500** | Internal Server Error |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## updateCluster

> updateCluster(name, dtoAerospikeCluster)

Updates an existing Aerospike cluster in the configuration.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { UpdateClusterRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Aerospike cluster name
    name: name_example,
    // DtoAerospikeCluster | Aerospike cluster details
    dtoAerospikeCluster: ...,
  } satisfies UpdateClusterRequest;

  try {
    const data = await api.updateCluster(body);
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
| **name** | `string` | Aerospike cluster name | [Defaults to `undefined`] |
| **dtoAerospikeCluster** | [DtoAerospikeCluster](DtoAerospikeCluster.md) | Aerospike cluster details | |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## updateConfig

> updateConfig(dtoConfig)

Updates the configuration for the service.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { UpdateConfigRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // DtoConfig | Configuration details
    dtoConfig: ...,
  } satisfies UpdateConfigRequest;

  try {
    const data = await api.updateConfig(body);
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
| **dtoConfig** | [DtoConfig](DtoConfig.md) | Configuration details | |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## updatePolicy

> updatePolicy(name, dtoBackupPolicy)

Updates an existing policy in the configuration.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { UpdatePolicyRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup policy name
    name: name_example,
    // DtoBackupPolicy | Backup policy details
    dtoBackupPolicy: ...,
  } satisfies UpdatePolicyRequest;

  try {
    const data = await api.updatePolicy(body);
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
| **name** | `string` | Backup policy name | [Defaults to `undefined`] |
| **dtoBackupPolicy** | [DtoBackupPolicy](DtoBackupPolicy.md) | Backup policy details | |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## updateRoutine

> updateRoutine(name, dtoBackupRoutine)

Updates an existing routine in the configuration.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { UpdateRoutineRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup routine name
    name: name_example,
    // DtoBackupRoutine | Backup routine details
    dtoBackupRoutine: ...,
  } satisfies UpdateRoutineRequest;

  try {
    const data = await api.updateRoutine(body);
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
| **dtoBackupRoutine** | [DtoBackupRoutine](DtoBackupRoutine.md) | Backup routine details | |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


## updateStorage

> updateStorage(name, dtoStorage)

Updates an existing storage in the configuration.

### Example

```ts
import {
  Configuration,
  ConfigurationApi,
} from '';
import type { UpdateStorageRequest } from '';

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new ConfigurationApi();

  const body = {
    // string | Backup storage name
    name: name_example,
    // DtoStorage | Backup storage details
    dtoStorage: ...,
  } satisfies UpdateStorageRequest;

  try {
    const data = await api.updateStorage(body);
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
| **name** | `string` | Backup storage name | [Defaults to `undefined`] |
| **dtoStorage** | [DtoStorage](DtoStorage.md) | Backup storage details | |

### Return type

`void` (Empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: `application/json`
- **Accept**: `*/*`


### HTTP response details
| Status code | Description | Response headers |
|-------------|-------------|------------------|
| **200** | OK |  -  |
| **400** | Bad Request |  -  |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

