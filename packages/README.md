# DEB package

## Install

### Installation
```shell
cd target && sudo dpkg -i aerospike-backup-service_0.1.0-1_amd64.deb
```

### Verify Installation
```shell
sudo systemctl status aerospike-backup-service
```

Should get the following output
```
● aerospike-backup-service.service - Aerospike Backup Service
     Loaded: loaded (/lib/systemd/system/aerospike-backup-service.service; enabled; vendor preset: enabled)
     Active: active (running) since Wed 2023-12-20 11:08:58 UTC; 14min ago
   Main PID: 229439 (aerospike-backu)
      Tasks: 26 (limit: 19160)
     Memory: 32.3M
        CPU: 6.562s
     CGroup: /system.slice/aerospike-backup-service.service
             └─229439 /usr/bin/aerospike-backup-service --config /etc/aerospike-backup-service/aerospike-backup-service.yml
```

### Modify service configuration
```shell
sudo vi /etc/aerospike-backup-service/aerospike-backup-service.yml
```

Restart service
```shell
sudo systemctl restart aerospike-backup-service
```

### Check service logs
```shell
sudo journalctl -u aerospike-backup-service -n 100 --no-page -f
```

## Uninstall

Stop the service
```shell
sudo systemctl stop aerospike-backup-service
```

### Remove service
* keeping the configuration
   ```shell
   sudo dpkg -r aerospike-backup-service
   ```

* completely
   ```shell
   sudo dpkg -P aerospike-backup-service
   ```

To remove the backup files run
```shell
sudo rm -rf /var/lib/aerospike-backup-service
```
