
# LogEntry

Entry represents a structured log entry.

## Properties

Name | Type
------------ | -------------
`attrs` | { [key: string]: any; }
`level` | string
`msg` | string
`time` | string

## Example

```typescript
import type { LogEntry } from ''

// TODO: Update the object below with actual values
const example = {
  "attrs": null,
  "level": INFO,
  "msg": Hello world!,
  "time": 2006-01-02T15:04:05Z07:00,
} satisfies LogEntry

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as LogEntry
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


