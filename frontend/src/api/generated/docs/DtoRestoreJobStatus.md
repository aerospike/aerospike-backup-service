# DtoRestoreJobStatus

RestoreJobStatus represents restore job status.

## Properties

| Name              | Type                              |
| ----------------- | --------------------------------- |
| `currentJob`      | [DtoRunningJob](DtoRunningJob.md) |
| `error`           | string                            |
| `errorsInDoubt`   | number                            |
| `existedRecords`  | number                            |
| `expiredRecords`  | number                            |
| `fresherRecords`  | number                            |
| `ignoredRecords`  | number                            |
| `indexCount`      | number                            |
| `insertedRecords` | number                            |
| `readRecords`     | number                            |
| `skippedRecords`  | number                            |
| `status`          | [DtoJobStatus](DtoJobStatus.md)   |
| `totalBytes`      | number                            |
| `udfCount`        | number                            |

## Example

```typescript
import type { DtoRestoreJobStatus } from "";

// TODO: Update the object below with actual values
const example = {
  currentJob: null,
  error: null,
  errorsInDoubt: 7,
  existedRecords: 15,
  expiredRecords: 2,
  fresherRecords: 5,
  ignoredRecords: 12,
  indexCount: 3,
  insertedRecords: 8,
  readRecords: 10,
  skippedRecords: 4,
  status: null,
  totalBytes: 2000,
  udfCount: 1,
} satisfies DtoRestoreJobStatus;

console.log(example);

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example);
console.log(exampleJSON);

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoRestoreJobStatus;
console.log(exampleParsed);
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
