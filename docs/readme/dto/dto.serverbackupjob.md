## dto.ServerBackupJob
ServerBackupJob is a server-side backup job of one namespace, as the server last reported it.

| Field                 | Description                                                                                   | Possible Values                 |
|-----------------------|-----------------------------------------------------------------------------------------------|---------------------------------|
| `abort-origin`        | What aborted the job: an operator request, a failure on a node or a cluster event.            | `operator`, `server`, `cluster` |
| `bytes-backed-up`     | The number of bytes backed up.                                                                |                                 |
| `error-reason`        | Why the job failed or was aborted, as the server reports it.                                  |                                 |
| `finish-time`         | The time the job finished.                                                                    |                                 |
| `job-id`              | The server's job id.                                                                          |                                 |
| `namespace`           | The namespace the job backed up.                                                              |                                 |
| `records-backed-up`   | The number of records backed up.                                                              |                                 |
| `records-read-failed` | The number of records the server could not read: a complete job that has any is missing them. |                                 |
| `start-time`          | The time the job started.                                                                     |                                 |
| `state`               | The server's job state, e.g. COMPLETE, FAILED or ABORTED.                                     |                                 |
