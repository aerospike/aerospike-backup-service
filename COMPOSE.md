# Docker-Compose Installation:
## Install:
```shell
 docker compose up -d 
```
## Verify Installation:
```
CONTAINER ID   IMAGE                                                                              COMMAND                  CREATED              STATUS                             PORTS                              NAMES
05e7e6247776   aerospike.jfrog.io/ecosystem-container-dev-local/aerospike-backup-service:latest   "./backup -c config.…"   About a minute ago   Up 53 seconds (health: starting)   0.0.0.0:8080->8080/tcp             aero-backup-service
7a359245eb3a   minio/minio:latest                                                                 "/usr/bin/docker-ent…"   About a minute ago   Up About a minute (healthy)        0.0.0.0:9000-9001->9000-9001/tcp   minio
bf93abfb953c   aerospike/aerospike-server-enterprise:6.2.0.7                                      "/usr/bin/as-tini-st…"   About a minute ago   Up About a minute (healthy)        0.0.0.0:3000-3003->3000-3003/tcp   aero-cluster
```
## View Logs
``` shell
docker logs -f <container-id> 
```
## Uninstall
```shell
 docker compose down 
```
