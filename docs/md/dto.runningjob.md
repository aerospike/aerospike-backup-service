## dto.RunningJob

| Field                | Description                                                                                                                                          |
|----------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| `done-records`       | DoneRecords: the number of records that have been successfully done.                                                                                 |
| `estimated-end-time` | EstimatedEndTime: the estimated time when the backup operation will be completed.<br>A nil value indicates that the estimation is not available yet. |
| `finish-time`        | FinishTime: the time when the operation finished.<br>A nil value indicates that the operation is still running.                                      |
| `metrics`            | Metrics provides real-time information about data flow performance.<br>See: [dto.Metrics](dto.metrics.md)                                            |
| `percentage-done`    | PercentageDone: the progress of the backup operation as a percentage.                                                                                |
| `start-time`         | StartTime: the time when the operation started.                                                                                                      |
| `total-records`      | TotalRecords: the total number of records to be processed.                                                                                           |
