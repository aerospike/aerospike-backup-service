
# DtoRunningJob

RunningJob tracks progress of currently running job.

## Properties

 Name               | Type                        
--------------------|-----------------------------
 `doneRecords`      | number                      
 `duration`         | number                      
 `estimatedEndTime` | string                      
 `finishTime`       | string                      
 `metrics`          | [DtoMetrics](DtoMetrics.md) 
 `percentageDone`   | number                      
 `startTime`        | string                      
 `totalRecords`     | number                      

## Example

```typescript
import type { DtoRunningJob } from ''

// TODO: Update the object below with actual values
const example = {
  "doneRecords": 50,
  "duration": null,
  "estimatedEndTime": 2006-01-02T15:04:05Z07:00,
  "finishTime": 2006-01-02T15:04:05Z07:00,
  "metrics": null,
  "percentageDone": 50,
  "startTime": 2006-01-02T15:04:05Z07:00,
  "totalRecords": 100,
} satisfies DtoRunningJob

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoRunningJob
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


