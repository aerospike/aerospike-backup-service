# SystemApi

All URIs are relative to _http://localhost:8080_

| Method                              | HTTP request       | Description                     |
| ----------------------------------- | ------------------ | ------------------------------- |
| [**apiDocs**](SystemApi.md#apidocs) | **GET** /api-docs/ | OpenAPI specification endpoint. |
| [**health**](SystemApi.md#health)   | **GET** /health    | Health endpoint.                |
| [**metrics**](SystemApi.md#metrics) | **GET** /metrics   | Prometheus metrics endpoint.    |
| [**ready**](SystemApi.md#ready)     | **GET** /ready     | Readiness endpoint.             |
| [**root**](SystemApi.md#root)       | **GET** /          | Root endpoint.                  |
| [**version**](SystemApi.md#version) | **GET** /version   | Returns application version.    |

## apiDocs

> string apiDocs()

OpenAPI specification endpoint.

Serves the API documentation in Swagger UI format.

### Example

```ts
import { Configuration, SystemApi } from "";
import type { ApiDocsRequest } from "";

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new SystemApi();

  try {
    const data = await api.apiDocs();
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

**string**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `text/html`

### HTTP response details

| Status code | Description | Response headers |
| ----------- | ----------- | ---------------- |
| **200**     | OK          | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## health

> health()

Health endpoint.

### Example

```ts
import { Configuration, SystemApi } from "";
import type { HealthRequest } from "";

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new SystemApi();

  try {
    const data = await api.health();
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
- **Accept**: Not defined

### HTTP response details

| Status code | Description | Response headers |
| ----------- | ----------- | ---------------- |
| **200**     | OK          | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## metrics

> metrics()

Prometheus metrics endpoint.

### Example

```ts
import { Configuration, SystemApi } from "";
import type { MetricsRequest } from "";

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new SystemApi();

  try {
    const data = await api.metrics();
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
- **Accept**: Not defined

### HTTP response details

| Status code | Description | Response headers |
| ----------- | ----------- | ---------------- |
| **200**     | OK          | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## ready

> ready()

Readiness endpoint.

### Example

```ts
import { Configuration, SystemApi } from "";
import type { ReadyRequest } from "";

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new SystemApi();

  try {
    const data = await api.ready();
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
- **Accept**: Not defined

### HTTP response details

| Status code | Description | Response headers |
| ----------- | ----------- | ---------------- |
| **200**     | OK          | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## root

> root()

Root endpoint.

### Example

```ts
import { Configuration, SystemApi } from "";
import type { RootRequest } from "";

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new SystemApi();

  try {
    const data = await api.root();
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
- **Accept**: Not defined

### HTTP response details

| Status code | Description | Response headers |
| ----------- | ----------- | ---------------- |
| **200**     | OK          | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)

## version

> string version()

Returns application version.

### Example

```ts
import { Configuration, SystemApi } from "";
import type { VersionRequest } from "";

async function example() {
  console.log("🚀 Testing  SDK...");
  const api = new SystemApi();

  try {
    const data = await api.version();
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

**string**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: `*/*`

### HTTP response details

| Status code | Description | Response headers |
| ----------- | ----------- | ---------------- |
| **200**     | version     | -                |

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
