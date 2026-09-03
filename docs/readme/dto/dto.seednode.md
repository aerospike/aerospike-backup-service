## dto.SeedNode
SeedNode represents details of a node in the Aerospike cluster.

| Field            | Description                                                                                                                                                                   |
|------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 📍 `host-name`    | The host name of the node.                                                                                                                                                    |
| 📍 `port`         | The port of the node.                                                                                                                                                         |
| `tls-name`       | TLS name sent as SNI and checked against the server certificate.<br>Required when the cluster has a tls block.<br>This is the name that takes effect for cluster connections. |

📍 = Required field