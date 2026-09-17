# Hardened service account and sandbox

The packages now run the service as the unprivileged system account
`aerospike-backup-service`, created by the package, under a systemd sandbox. The points
below are the ones that change behaviour for an existing installation.

## Paths the service can write

`ProtectSystem=strict` makes the filesystem read-only except for:

| Path | Created by | Mode |
| --- | --- | --- |
| `/etc/aerospike-backup-service` | package | config file `0640` |
| `/var/lib/aerospike-backup-service` | `StateDirectory=` | `0750` |
| `/var/log/aerospike-backup-service` | `LogsDirectory=` | `0750` |

A backup routine whose `local-storage` path points anywhere else fails with
`read-only file system`. Add the path with a drop-in:

```shell
sudo systemctl edit aerospike-backup-service
```

```ini
[Service]
ReadWritePaths=/srv/backups
```

```shell
sudo chown -R aerospike-backup-service:aerospike-backup-service /srv/backups
sudo systemctl daemon-reload && sudo systemctl restart aerospike-backup-service
```

A worked example ships at
`/usr/share/doc/aerospike-backup-service/local-storage-path.conf.example`.

## Unit file location

The unit ships at `/usr/lib/systemd/system/aerospike-backup-service.service` and is
vendor-owned: package upgrades always replace it. Customisation belongs in a drop-in
(`systemctl edit aerospike-backup-service`), which `/etc` still overrides cleanly.

Earlier releases shipped it as a conffile in `/etc/systemd/system`. If you had edited
that copy, it would have kept overriding the hardened unit, so the postinstall script
moves it to `aerospike-backup-service.service.pre-hardening.bak` and says so. Re-apply
any local changes as a drop-in.

**Downgrading** to a pre-hardening package therefore needs a purge first — dpkg does not
restore a conffile that is no longer present, so the old package's `postinst` fails on
`systemctl enable`:

```shell
sudo apt-get purge -y aerospike-backup-service
sudo apt-get install -y ./aerospike-backup-service_<old-version>_amd64.deb
```

## Credentials

`ProtectHome=true` hides `/root` and `/home`, and the service account's home is
`/var/lib/aerospike-backup-service`. Cloud credentials that used to sit in
`/root/.aws/credentials` are no longer reachable. Either move them under the service
account's home:

```shell
sudo install -d -o aerospike-backup-service -g aerospike-backup-service -m 0700 \
    /var/lib/aerospike-backup-service/.aws
sudo install -o aerospike-backup-service -g aerospike-backup-service -m 0600 \
    /root/.aws/credentials /var/lib/aerospike-backup-service/.aws/credentials
```

or supply them through the environment, which the unit reads from
`/etc/default/aerospike-backup-service` (deb) or `/etc/sysconfig/aerospike-backup-service`
(rpm):

```shell
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
```

TLS keys, CA files and `password-path` files must be readable by the service account —
a `0600 root:root` key is not. Prefer `0640 root:aerospike-backup-service`.

## File modes

`UMask=0027` means backup artifacts are created `0640`, directories `0750`, owned by
`aerospike-backup-service`. They are no longer world-readable. To let an operator
account read them, add it to the group:

```shell
sudo usermod -aG aerospike-backup-service alice
```

## Ports below 1024

The unit drops every capability, so `service.http.port` under 1024 fails to bind. Grant
the one capability back with a drop-in:

```ini
[Service]
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
```

## Logs

Logs move from `/var/log/aerospike-backup-service.log` to
`/var/log/aerospike-backup-service/aerospike-backup-service.log`. The postinstall script
moves an existing log and its rotated siblings into the new directory. If you kept a
customised configuration file, update `service.logger.file-writer.filename` to match —
the old path is no longer writable.

On el8 (systemd 239) `ProtectKernelLogs`, `ProtectClock` and `ProtectHostname` are not
recognised and are ignored; the rest of the sandbox applies.

# DEB package

## Install

### Installation

```shell
sudo dpkg -i aerospike-backup-service_3.6.1-1_amd64.deb
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

# RPM Package

## Install

### Installation

```shell
sudo rpm -i aerospike-backup-service-3.6.1-1.x86_64.rpm
```

### Verify Installation

```shell
sudo systemctl status aerospike-backup-service
```

Should get the following output

```
● aerospike-backup-service.service - Aerospike Backup Service
     Loaded: loaded (/usr/lib/systemd/system/aerospike-backup-service.service; enabled; preset: disabled)
     Active: active (running) since Mon 2023-12-25 14:07:10 UTC; 2min 3s ago
   Main PID: 182440 (aerospike-backu)
      Tasks: 9 (limit: 19154)
     Memory: 6.6M
        CPU: 1.918s
     CGroup: /system.slice/aerospike-backup-service.service
             └─182440 /usr/bin/aerospike-backup-service --config /etc/aerospike-backup-service/aerospike-backup-service.yml
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

```shell
sudo rpm -e aerospike-backup-service
```

To remove the backup files run

```shell
sudo rm -rf /var/lib/aerospike-backup-service
```
