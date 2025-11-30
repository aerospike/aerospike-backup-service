# DtoBackupRoutine

BackupRoutine represents a scheduled backup operation routine.

## Properties

| Name               | Type                |
| ------------------ | ------------------- |
| `backupPolicy`     | string              |
| `binList`          | Array&lt;string&gt; |
| `disabled`         | boolean             |
| `incrIntervalCron` | string              |
| `intervalCron`     | string              |
| `namespaces`       | Array&lt;string&gt; |
| `nodeList`         | Array&lt;string&gt; |
| `partitionList`    | string              |
| `rackList`         | Array&lt;number&gt; |
| `secretAgent`      | string              |
| `setList`          | Array&lt;string&gt; |
| `sourceCluster`    | string              |
| `storage`          | string              |

## Example

```typescript
import type { DtoBackupRoutine } from ''

// TODO: Update the object below with actual values
const example = {
  "backupPolicy": null,
  "binList": ["dataBin"],
  "disabled": null,
  "incrIntervalCron": */10 * * * * *,
  "intervalCron": 0 0 * * * *,
  "namespaces": ["[\"source-ns1\"]"],
  "nodeList": null,
  "partitionList": null,
  "rackList": null,
  "secretAgent": null,
  "setList": ["set1"],
  "sourceCluster": testCluster,
  "storage": null,
} satisfies DtoBackupRoutine

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as DtoBackupRoutine
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)
