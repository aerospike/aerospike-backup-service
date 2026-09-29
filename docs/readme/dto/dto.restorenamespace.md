## dto.RestoreNamespace
RestoreNamespace specifies an alternative namespace name for the restore operation.

| Field              | Description                                                                                                                                                                                                                                                  |
|--------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 📍 `destination`    | Name of the destination namespace to restore data into.<br>A name follows the Aerospike naming rules: at most 31 bytes of Latin letters, digits,<br>"_", "-" and "$", and not the reserved name "null".                                                      |
| 📍 `source`         | Original namespace name.<br>This field is required as a safeguard to ensure intentional namespace remapping.<br>A name follows the Aerospike naming rules: at most 31 bytes of Latin letters, digits,<br>"_", "-" and "$", and not the reserved name "null". |

📍 = Required field