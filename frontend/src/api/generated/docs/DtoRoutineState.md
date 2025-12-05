
# DtoRoutineState

RoutineState represent the current state of backups (full and incremental).

## Properties

Name | Type
------------ | -------------
`full` | [DtoRunningJob](DtoRunningJob.md)
`incremental` | [DtoRunningJob](DtoRunningJob.md)
`lastFull` | string
`lastIncremental` | string
`nextFull` | string
`nextIncremental` | string

## Example

```typescript
import type { DtoRoutineState } from ''

// TODO: Update the object below with actual values
const example = {
  "full": null,
  "incremental": null,
  "lastFull": null,
  "lastIncremental": null,
  "nextFull": null,
  "nextIncremental": null,
} satisfies DtoRoutineState

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoRoutineState
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


