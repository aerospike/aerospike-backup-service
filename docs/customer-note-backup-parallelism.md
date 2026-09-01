# Backup scan behavior: legacy asbackup vs absctl

When running a backup with **`parallel=1`**, the two tools do not behave the same on a multi-node cluster:

- **Legacy asbackup** scans **one node at a time**. Work moves from node to node until the backup completes.
- **absctl** (the new backup CLI) issues scan work to **all nodes at the same time**, even when `parallel` is set to `1`.

Both tools use `parallel` to control how much scan work runs in parallel; 

For questions about tuning backups on your cluster, contact your Aerospike support representative.
