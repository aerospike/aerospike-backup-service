# REST API Specification (v0.1.0)

## Endpoints

### System

<details>
  <summary><code>GET /</code></summary>
  Root endpoint.
</details>
<details>
  <summary><code>GET /health</code></summary>
  Health endpoint.
</details>
<details>
  <summary><code>GET /ready</code></summary>
  Readiness endpoint.
</details>
<details>
  <summary><code>GET /metrics</code></summary>
  Prometheus metrics endpoint.
</details>
<details>
  <summary><code>GET /version</code></summary>
  Returns application version.
</details>

### Application

<details>
  <summary><code>GET /config</code></summary>
  Returns the configuration the service started with in the JSON format.
</details>
<details>
  <summary><code>POST /restore</code></summary>
  Trigger an asynchronous restore operation.
  <hr/>
  Request (<i>model.RestoreRequest</i>):

  ```json
  {
    "host": "localhost",
    "port": 3000,
    "directory": "./testout/backup",
    "set_list": ["set1"]
  }
  ```
  <sup>*</sup> Specify the `directory` parameter for the full backup restore. 
  Use the `file` parameter to restore from an incremental backup file.

  Response - Job ID (<i>int64</i>):

  ```
  8434960093091118956
  ```
</details>

<details>
  <summary><code>GET /restore/status?jobId={job_id}</code></summary>
  Retrive status for a restore job.
  <hr/>
  Query parameters:<br/>

  * job_id (<i>int64</i>)

  Response - Job status (<i>string</i>):

  ```
  DONE
  ```
</details>

<details>
  <summary><code>GET /backup/full/list?name={policy_name}</code></summary>
  Retrieve a list of available full backups.
  <hr/>
  Query parameters:<br/>

  * policy_name (<i>string</i>)

  Response (<i>[]model.BackupDetails</i>):

  ```json
  [
    {
      "key": "1695572892",
      "last_modified": "2023-09-24T19:28:12.658312699+03:00",
      "size": 96
    }
  ]
  ```
</details>

<details>
  <summary><code>GET /backup/incremental/list?name={policy_name}</code></summary>
  Retrieve a list of available incremental backups.
  <hr/>
  Query parameters:<br/>

  * policy_name (<i>string</i>)

  Response (<i>[]model.BackupDetails</i>):

  ```json
  [
    {
      "key": "1695745321.asb",
      "last_modified": "2023-09-26T19:22:01.865810616+03:00",
      "size": 2443
    }
  ]
  ```
</details>

## Models

</details>
<details>
  <summary><code>model.BackupDetails</code></summary>

  | Field         | Type      | Description  
  | ---           | ---       | ---
  | key           | string    | The relative path to the backup folder or file
  | last_modified | time.Time | The last modified time
  | size          | int64     | The size of the backup file/folder

</details>

</details>
<details>
  <summary><code>model.RestoreRequest</code></summary>

  | Field                | Type     | Description  
  | ---                  | ---      | ---
  | host                 | string   | The host of the Aerospike cluster to restore to
  | port                 | int      | The port of the Aerospike cluster to restore to
  | user                 | string   | The Aerospike user
  | password             | string   | The Aerospike password
  | directory            | string   | The directory to restore from
  | file                 | string   | The file to restore from
  | s3_region            | string   | The region to use for S3
  | s3_profile           | string   | The profile to use for AWS credentials
  | s3_endpoint_override | string   | An alternative endpoint for S3 compatible storage to send all S3 requests to
  | s3_log_level         | string   | Logging level of the AWS S3 C+ SDK
  | ns_list              | []string | The (optional) source and (also optional) target namespace to be restored
  | set_list             | []string | The sets to be restored
  | bin_list             | []string | The bins to be restored
  | replace              | bool     | Indicates that existing records should be replaced instead of updated
  | unique               | bool     | Indicates that existing records shouldn't be touched
  | no_generation        | bool     | Indicates that the generation count of existing records should be ignored

</details>
