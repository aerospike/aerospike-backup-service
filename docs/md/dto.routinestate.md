## dto.RoutineState

| Field              | Description                                                                                                                                     |
|--------------------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| `full`             | Full represents the state of a full backup. Nil if no full backup is running.<br>See: [dto.RunningJob](dto.runningjob.md)                       |
| `incremental`      | Incremental represents the state of an incremental backup. Nil if no incremental backup is running.<br>See: [dto.RunningJob](dto.runningjob.md) |
| `last-full`        | LastFull: the time of the last successful full backup.<br>A nil value indicates that there has never been a full backup.                        |
| `last-incremental` | LastIncremental: the time of the last successful incremental backup.<br>A nil value indicates that there has never been an incremental backup.  |
| `next-full`        | NextFull: the time of the next scheduled full backup.                                                                                           |
| `next-incremental` | NextIncremental: the time of the next scheduled incremental backup.                                                                             |
