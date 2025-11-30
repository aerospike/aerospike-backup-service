# DtoLocalStorage

LocalStorage represents the configuration for local storage.

## Properties

| Name          | Type   |
| ------------- | ------ |
| `minPartSize` | number |
| `path`        | string |

## Example

```typescript
import type { DtoLocalStorage } from "";

// TODO: Update the object below with actual values
const example = {
  minPartSize: null,
  path: backups,
} satisfies DtoLocalStorage;

console.log(example);

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example);
console.log(exampleJSON);

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoLocalStorage;
console.log(exampleParsed);
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
