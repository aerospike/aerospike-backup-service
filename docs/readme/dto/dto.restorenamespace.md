## dto.RestoreNamespace
RestoreNamespace specifies an alternative namespace name for the restore operation.

| Field              | Description                                                                                                  |
|--------------------|--------------------------------------------------------------------------------------------------------------|
| 🔴 `destination`    | Name of the destination namespace to restore data into.                                                      |
| 🔴 `source`         | Original namespace name.<br>This field is required as a safeguard to ensure intentional namespace remapping. |

🔴 = Required field