# DtoLoggerConfig

LoggerConfig represents the backup service logger configuration.

## Properties

| Name           | Type                                          |
| -------------- | --------------------------------------------- |
| `fileWriter`   | [DtoFileLoggerConfig](DtoFileLoggerConfig.md) |
| `format`       | string                                        |
| `level`        | string                                        |
| `stdoutWriter` | boolean                                       |

## Example

```typescript
import type { DtoLoggerConfig } from "";

// TODO: Update the object below with actual values
const example = {
  fileWriter: null,
  format: null,
  level: null,
  stdoutWriter: null,
} satisfies DtoLoggerConfig;

console.log(example);

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example);
console.log(exampleJSON);

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoLoggerConfig;
console.log(exampleParsed);
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
