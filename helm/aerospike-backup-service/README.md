# Aerospike Backup Service

This Helm chart runs the official [Aerospike Backup Service](https://github.com/aerospike/aerospike-backup-service)
(ABS) Docker image on a Kubernetes cluster. It creates a `Deployment`, a `Service`, a `ServiceAccount`, a `ConfigMap`
holding the service configuration, and optionally an `Ingress` and a `HorizontalPodAutoscaler`.

Full service documentation, including every configuration field, is in the
[project docs](https://github.com/aerospike/aerospike-backup-service/tree/main/docs).

## Prerequisites

- Kubernetes 1.23 or later.
- Helm v3 or later.
- An Aerospike cluster reachable from the pods. It can run in the same Kubernetes cluster using the
  [Aerospike Kubernetes Operator](https://aerospike.com/docs/kubernetes/).

## Versions

| Chart  | Aerospike Backup Service (`appVersion`, default `image.tag`) |
|--------|--------------------------------------------------------------|
| 2.1.x  | 3.7.x                                                        |
| 2.0.11 | 3.6.1                                                        |
| 2.0.10 | 3.6.0                                                        |

From ABS 3.7.0, the chart minor version advances once per ABS minor line and the chart patch version matches the ABS
patch version. To pin an install, pass `--version <chart version>` to `helm install` and `helm upgrade`.

## Adding the helm chart repository

Add the `aerospike` helm repository if not already done

```shell
helm repo add aerospike https://aerospike.github.io/helm-charts
helm repo update
```

## Quick start

Create a values file, for example `abs-values.yaml`. The `backupServiceConfig` key holds the ABS
[configuration file](https://github.com/aerospike/aerospike-backup-service/blob/main/docs/configuration.md); the rest
configures Kubernetes. This example backs up namespace `test` to S3 every night and reads the Aerospike password from a
Kubernetes Secret:

```yaml
backupServiceConfig:
  aerospike-clusters:
    main:
      seed-nodes:
        - host-name: aerocluster.aerospike.svc.cluster.local
          port: 3000
      credentials:
        user: backup
        password-path: /etc/abs-secrets/aerospike-password
  storage:
    s3:
      s3-storage:
        bucket: my-backup-bucket
        s3-region: us-east-1
        path: abs
  backup-policies:
    keep-week:
      retention:
        full: 7
  backup-routines:
    nightly:
      source-cluster: main
      storage: s3
      backup-policy: keep-week
      interval-cron: "@daily"
      incr-interval-cron: "@hourly"
      namespaces: [ "test" ]

volumes:
  - name: aerospike-password
    secret:
      secretName: abs-aerospike
extraSecretVolumeMounts:
  - name: aerospike-password
    mountPath: /etc/abs-secrets/aerospike-password
    subPath: password
```

S3 credentials for this example come from the default AWS chain; see [Cloud storage credentials](#cloud-storage-credentials).

Create the namespace and the Secret, then install:

```shell
kubectl create namespace aerospike
kubectl -n aerospike create secret generic abs-aerospike --from-literal=password='<aerospike password>'

# helm install --namespace <target namespace> <release name> -f <values file> aerospike/aerospike-backup-service
helm install abs aerospike/aerospike-backup-service \
  --namespace aerospike \
  --values abs-values.yaml \
  --wait
```

Here `abs` is the release name. Resources are named `<release name>-aerospike-backup-service`
(here `abs-aerospike-backup-service`) unless the release name already contains the chart name, or you set
`nameOverride` or `fullnameOverride`.

On success, Helm prints the release status and the chart notes, which show how to reach the API:

```text
NAME: abs
NAMESPACE: aerospike
STATUS: deployed
REVISION: 1
NOTES:
1. Get the application URL by running these commands:
  export POD_NAME=$(kubectl get pods --namespace aerospike -l "app.kubernetes.io/name=aerospike-backup-service,app.kubernetes.io/instance=abs" -o jsonpath="{.items[0].metadata.name}")
  ...
```

To check the release, run `helm test abs -n aerospike`. It starts a short-lived pod that requests the `health` endpoint
through the Service.

## Configuration

| Parameter                 | Description                                                                                                                                                                                                                                                      | Default                                                     |
|---------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------|
| `replicaCount`            | Number of ABS pods. Keep at `1`; see [Replicas and scaling](#replicas-and-scaling).                                                                                                                                                                              | `1`                                                         |
| `image`                   | ABS image `repository`, `tag` and `pullPolicy`. `tag` falls back to the chart `appVersion` when empty.                                                                                                                                                           | `aerospike/aerospike-backup-service:3.7.0`, `Always`        |
| `imagePullSecrets`        | Secrets for pulling from a private registry.                                                                                                                                                                                                                     | `[]`                                                        |
| `nameOverride`            | Replaces the chart name in resource names and the `app.kubernetes.io/name` label.                                                                                                                                                                                | `""`                                                        |
| `fullnameOverride`        | Replaces the full resource name.                                                                                                                                                                                                                                 | `""`                                                        |
| `utilityImage`            | Image used by the `config-injector` init container (when `configmap.create` is `false`) and by the `helm test` pod. Needs `/bin/sh`, `base64` and `wget`.                                                                                                       | `busybox:1.37.0`, `IfNotPresent`                            |
| `configmap.create`        | How `backupServiceConfig` reaches the pod. See [Configuration source](#configuration-source).                                                                                                                                                                   | `true`                                                      |
| `args`                    | ABS command-line arguments. Empty means `-c /etc/aerospike-backup-service/aerospike-backup-service.yml`. A non-empty list replaces that default, so include `-c` yourself. Add `-r` to treat the mounted file as a [remote configuration](#remote-configuration) pointer. | `[]`                                                        |
| `env`                     | Extra container environment variables, for example `valueFrom.secretKeyRef`.                                                                                                                                                                                    | `[]`                                                        |
| `envFrom`                 | Import all keys of a ConfigMap or Secret as environment variables.                                                                                                                                                                                               | `[]`                                                        |
| `backupServiceConfig`     | ABS configuration, written to `/etc/aerospike-backup-service/aerospike-backup-service.yml`. Unknown keys fail startup. See the commented reference in [values.yaml](values.yaml) and the [field reference](https://github.com/aerospike/aerospike-backup-service/tree/main/docs/readme/dto). | `{}`                                                        |
| `initContainers`          | Extra init containers, run after `config-injector`.                                                                                                                                                                                                              | `[]`                                                        |
| `serviceAccount`          | `create`, `name` and `annotations` (for IRSA or Workload Identity). `automount` sets the field on the ServiceAccount object only: the pod always sets `automountServiceAccountToken: false`.                                                                    | `create: true`, `automount: false`                          |
| `podAnnotations`          | Extra pod [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/).                                                                                                                                                          | `{}`                                                        |
| `podLabels`               | Extra pod [labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/).                                                                                                                                                                    | `{}`                                                        |
| `podSecurityContext`      | Pod [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/).                                                                                                                                                              | `runAsNonRoot: true`, `fsGroup: 65532`, seccomp `RuntimeDefault` |
| `securityContext`         | Container [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container). Hardened: non-root uid/gid 65532, `readOnlyRootFilesystem`, no privilege escalation, all capabilities dropped, seccomp `RuntimeDefault`. | see [values.yaml](values.yaml)                              |
| `service.type`            | Kubernetes Service type.                                                                                                                                                                                                                                         | `ClusterIP`                                                 |
| `service.port`            | Service port, target port and container port. Must equal `backupServiceConfig.service.http.port`. See [Port, context path and probes](#port-context-path-and-probes).                                                                                          | `8080`                                                      |
| `service.httpsPort`       | HTTPS port, added to the container and the Service as port `https` when set. Must equal `backupServiceConfig.service.https.port`. See [HTTPS](#https).                                                                                                        | unset                                                       |
| `ingress`                 | Optional Ingress: `enabled`, `className`, `annotations`, `hosts[].paths[]`, `tls`.                                                                                                                                                                               | `enabled: false`                                            |
| `resources`               | Container [requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/).                                                                                                                                                  | `{}`                                                        |
| `livenessProbe`           | Liveness probe.                                                                                                                                                                                                                                                  | `GET /health` on port `8080`, delay 15s, period 10s         |
| `readinessProbe`          | Readiness probe.                                                                                                                                                                                                                                                 | `GET /ready` on port `8080`, delay 15s, period 10s          |
| `autoscaling`             | HorizontalPodAutoscaler (`autoscaling/v2`). Not recommended; see [Replicas and scaling](#replicas-and-scaling).                                                                                                                                                 | `enabled: false`                                            |
| `volumes`                 | Extra pod volumes (Secrets, PVCs, ConfigMaps, ...).                                                                                                                                                                                                              | `[]`                                                        |
| `extraVolumeMounts`       | Mounts for entries in `volumes`: `name`, `mountPath`, `readOnly`. `readOnly` defaults to `true`; booleans and the strings `"true"`/`"false"` are accepted.                                                                                                     | `[]`                                                        |
| `extraSecretVolumeMounts` | Like `extraVolumeMounts`, plus `subPath` to mount one key of a Secret as a file.                                                                                                                                                                                 | `[]`                                                        |
| `nodeSelector`            | [Node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector).                                                                                                                                                          | `{}`                                                        |
| `tolerations`             | [Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/).                                                                                                                                                                    | `[]`                                                        |
| `affinity`                | [Affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity) rules.                                                                                                                                           | `{}`                                                        |

### Configuration source

- **`configmap.create: true` (default).** `backupServiceConfig` is rendered into the ConfigMap
  `<fullname>-cm` and mounted read-only. The configuration REST API cannot change it: write requests return
  `405 Method Not Allowed`. Change `backupServiceConfig` and run `helm upgrade` instead.
- **`configmap.create: false`.** The `config-injector` init container (image `utilityImage`) writes
  `backupServiceConfig` into an `emptyDir`, so the configuration REST API can change it. Those changes live only in
  the pod: they are lost when the pod is recreated, including on every `helm upgrade` that changes the pod. Copy any
  change you want to keep back into your values file.

### Remote configuration

With `args: ["-c", "/etc/aerospike-backup-service/aerospike-backup-service.yml", "-r"]`, the mounted file does not
hold the configuration itself but the location of it, for example an S3 object. `backupServiceConfig` then contains
only that pointer:

```yaml
args: ["-c", "/etc/aerospike-backup-service/aerospike-backup-service.yml", "-r"]
backupServiceConfig:
  s3-storage:
    bucket: my-config-bucket
    path: config.yml
    s3-region: eu-central-1
```

See [Installation](https://github.com/aerospike/aerospike-backup-service/blob/main/docs/installation.md) for the remote
configuration format.

### Port, context path and probes

- `service.port` sets the container port, the Service port and target port, and the probe port. It does not change the
  port ABS listens on. If you change it, also set `backupServiceConfig.service.http.port` to the same value, and set
  `livenessProbe.httpGet.port` and `readinessProbe.httpGet.port`: in `values.yaml` the probes share the default through
  a YAML anchor, which does not apply to your own values file.
- If you set `backupServiceConfig.service.http.context-path` (for example `/abs`), prefix the probe paths with it
  (`/abs/health`, `/abs/ready`). The chart notes and `helm test` add the context path for you.
- The probes, the chart notes and `helm test` use plain HTTP on `service.port`. Keep the HTTP listener enabled when
  you also enable [HTTPS](#https).

### Writable paths (`readOnlyRootFilesystem`)

The default container `securityContext` sets `readOnlyRootFilesystem: true`. The chart mounts an `emptyDir` at `/tmp`;
everything else in the image, including `/var/log/aerospike-backup-service`, is read-only. Local backup storage and a
log file (`service.logger.file-writer`) need a writable volume. Use a
PersistentVolumeClaim for local storage: an `emptyDir` is deleted with the pod, and the backups with it.

```yaml
backupServiceConfig:
  storage:
    local:
      local-storage:
        path: /backup
volumes:
  - name: backups
    persistentVolumeClaim:
      claimName: abs-backups # create this PVC first
extraVolumeMounts:
  - name: backups
    mountPath: /backup
    readOnly: false
```

The pod's `fsGroup: 65532` makes the volume writable for the ABS user on most storage classes. Logs go to standard
output by default (`service.logger.stdout-writer: true`), which is what `kubectl logs` reads; this is the recommended
setup on Kubernetes. Override `securityContext.readOnlyRootFilesystem` only if you cannot mount a volume.

## Secrets and credentials

Keep credentials out of `backupServiceConfig`: the chart stores it in a ConfigMap, which is not encrypted and is
readable by anyone who can read ConfigMaps in the namespace. Instead, put them in Kubernetes Secrets and either mount
them as files (`volumes` with `extraSecretVolumeMounts`) or expose them as environment variables (`env`). ABS can also
read secrets from an [Aerospike Secret Agent](https://github.com/aerospike/aerospike-backup-service/blob/main/docs/security.md).

### Aerospike password

Shown in the [Quick start](#quick-start): mount the Secret key as a file and point `credentials.password-path` at it.

### TLS to Aerospike

```shell
kubectl -n aerospike create secret generic aerospike-tls \
  --from-file=ca.pem --from-file=cert.pem --from-file=key.pem
```

```yaml
volumes:
  - name: aerospike-tls
    secret:
      secretName: aerospike-tls
extraVolumeMounts:
  - name: aerospike-tls
    mountPath: /etc/abs-secrets/tls
backupServiceConfig:
  aerospike-clusters:
    main:
      seed-nodes:
        - host-name: aerocluster.aerospike.svc.cluster.local
          port: 4333
          tls-name: aerocluster
      tls:
        name: aerocluster
        ca-file: /etc/abs-secrets/tls/ca.pem
        cert-file: /etc/abs-secrets/tls/cert.pem # for PKI or mutual TLS
        key-file: /etc/abs-secrets/tls/key.pem
```

### Cloud storage credentials

**S3.** Without `access-key-id`/`secret-access-key`, ABS uses the default AWS credential chain. On EKS, use
[IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) or EKS Pod Identity:

```yaml
serviceAccount:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::<account-id>:role/<role-name>
```

Elsewhere, mount an AWS credentials file at the ABS user's home directory and select a profile with `s3-profile`:

```shell
kubectl -n aerospike create secret generic s3-credentials --from-file=credentials=./aws-credentials
```

```yaml
volumes:
  - name: s3-credentials
    secret:
      secretName: s3-credentials
extraSecretVolumeMounts:
  - name: s3-credentials
    mountPath: /home/absuser/.aws/credentials
    subPath: credentials
```

**GCP.** Without `key-file-path` or `key`, ABS uses Application Default Credentials, which works with
[GKE Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
(`serviceAccount.annotations: {iam.gke.io/gcp-service-account: <gsa>@<project>.iam.gserviceaccount.com}`).
Otherwise mount a service account key from a Secret and set `key-file-path` to it.

**Azure.** Without `account-name`/`account-key` or `tenant-id`/`client-id`/`client-secret`, ABS uses the Azure default
credential chain, which includes [Workload Identity](https://learn.microsoft.com/azure/aks/workload-identity-overview)
(set the `azure.workload.identity/client-id` ServiceAccount annotation and the `azure.workload.identity/use: "true"`
pod label through `podLabels`).

Values such as `secret-access-key`, `account-key` or `client-secret` can also reference a Secret Agent with
`secrets:<resource>:<key>`.

### Encryption keys

Pass the key from a Secret through an environment variable and reference it with `key-env`:

```yaml
env:
  - name: ABS_ENCRYPTION_KEY
    valueFrom:
      secretKeyRef:
        name: abs-encryption-key
        key: key.pem
backupServiceConfig:
  backup-policies:
    encrypted:
      encryption:
        mode: AES256
        key-env: ABS_ENCRYPTION_KEY
```

Alternatively mount the Secret as a file and use `key-file`.

## Exposing and securing the API

The REST API can change the configuration, start and cancel backups, and restore data. It has no user or token
authentication. The only built-in client authentication is mutual TLS on the HTTPS listener. Keep the default
`ClusterIP` Service and reach the API with `kubectl port-forward` (see the chart notes), or restrict access:

- Use a [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) to allow only the
  clients that need the API.
- If you enable `ingress`, put authentication in front of it (for example your ingress controller's auth annotations)
  and terminate TLS (`ingress.tls`).
- `backupServiceConfig.service.http.rate.white-list` exempts listed IPs from the rate limiter; it does not restrict access.

### HTTPS

To serve HTTPS, mount the server certificate and key (and a client CA for mTLS) from a Secret, configure
`service.https`, and expose the port with `service.httpsPort`:

```yaml
service:
  httpsPort: 8443 # must equal backupServiceConfig.service.https.port
volumes:
  - name: abs-https
    secret:
      secretName: abs-https # tls.crt, tls.key, ca.crt
extraVolumeMounts:
  - name: abs-https
    mountPath: /etc/abs-secrets/https
backupServiceConfig:
  service:
    https:
      port: 8443
      cert-file: /etc/abs-secrets/https/tls.crt
      key-file: /etc/abs-secrets/https/tls.key
      client-auth: require-and-verify # mutual TLS
      client-ca-file: /etc/abs-secrets/https/ca.crt
```

The Service then has a second port named `https` next to `http`. The `ingress` backend, the probes, the chart notes
and `helm test` still use the HTTP port. To route an Ingress to HTTPS, point your own Ingress at the `https`
Service port and configure your controller for an HTTPS backend. Mount the Secret as a directory (not with `subPath`) so certificate rotation reaches the pod;
ABS reloads rewritten certificate files without a restart. See
[Security](https://github.com/aerospike/aerospike-backup-service/blob/main/docs/security.md).

## Monitoring

ABS serves Prometheus metrics at `<context-path>metrics` (default `/metrics`) on the HTTP port. The Service names that
port `http`. With the Prometheus Operator, a ServiceMonitor such as this one scrapes it:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: abs
  namespace: aerospike
spec:
  selector:
    matchLabels:
      app.kubernetes.io/instance: abs
      app.kubernetes.io/name: aerospike-backup-service
  endpoints:
    - port: http
      path: /metrics
```

With annotation-based scraping, set `podAnnotations` (`prometheus.io/scrape: "true"`, `prometheus.io/port: "8080"`,
`prometheus.io/path: /metrics`). See [Monitoring](https://github.com/aerospike/aerospike-backup-service/blob/main/docs/monitoring.md)
for the available metrics.

## Operations

### List pods

```shell
# kubectl get pods --namespace aerospike --selector=app=<fullname>
kubectl get pods --namespace aerospike --selector=app=abs-aerospike-backup-service
```

```text
NAME                                            READY   STATUS    RESTARTS   AGE
abs-aerospike-backup-service-544cc664c9-g5gdj   1/1     Running   0          22m
```

### Get logs

```shell
# Skip -f for a one-time dump
kubectl -n aerospike logs -f deployment/abs-aerospike-backup-service
```

### Updating the configuration

Edit `backupServiceConfig` in your values file and upgrade the release:

```shell
# helm upgrade --namespace <target namespace> <release name> -f <values file> aerospike/aerospike-backup-service
helm upgrade --namespace aerospike abs -f abs-values.yaml aerospike/aerospike-backup-service
```

A change to `backupServiceConfig` changes a checksum annotation on the pod, so the pod is replaced and starts with
the new configuration. Keep in mind:

- Backups and restores running on the old pod are interrupted. On shutdown ABS waits up to 30 seconds, which matches
  the Kubernetes default termination grace period. Schedule upgrades outside backup windows.
- Only `backupServiceConfig` is tracked. After rotating a Secret or ConfigMap used through `env`, `envFrom` or a
  `subPath` mount, restart the pod yourself:
  `kubectl -n aerospike rollout restart deployment/abs-aerospike-backup-service`.
  Secrets mounted as whole directories are updated in place by the kubelet; see
  [Security](https://github.com/aerospike/aerospike-backup-service/blob/main/docs/security.md) for when ABS picks up
  each kind of credential.
- If ABS rejects the new configuration, the new pod does not become ready; see [Troubleshooting](#troubleshooting).

### Replicas and scaling

ABS has no leader election or coordination between instances. Every pod loads the same configuration and runs every
backup routine on its own schedule, so with more than one replica each backup runs once per pod, against the same
cluster and the same storage. Keep `replicaCount: 1` and `autoscaling.enabled: false`. To spread load, run separate
releases with different routines.

Scale vertically instead, with `resources`. If you enable `autoscaling` anyway, its utilization targets need
`resources.requests` to be set.

### Uninstall

```shell
helm uninstall abs --namespace aerospike
```

This removes the chart's resources. PersistentVolumeClaims and Secrets you created yourself, and backups in remote
storage, are left in place.

## Troubleshooting

### Pods not listed

Check for error events on the Deployment:

```shell
kubectl -n aerospike describe deployment abs-aerospike-backup-service
```

### Pods stuck in `Init`, `Pending` or `ContainerCreating`

```shell
kubectl -n aerospike describe pod <pod name>
```

Common causes shown in the events:

- A Secret, ConfigMap or PersistentVolumeClaim referenced in `volumes` does not exist in the release namespace.
- An unbound PersistentVolumeClaim, or no node matching `nodeSelector`, `affinity` or `tolerations`.
- The image (or `utilityImage`) cannot be pulled; check `imagePullSecrets`.
- The namespace enforces a Pod Security admission level the pod does not meet.

### Pods crash or never become ready

The most likely cause is an invalid `backupServiceConfig`: unknown keys and invalid values stop ABS at startup.
[Check the logs](#get-logs), fix the values file and [upgrade](#updating-the-configuration). If ABS starts but the
pod is not ready, check that `service.port`, `backupServiceConfig.service.http.port`, the context path and the probes
agree (see [Port, context path and probes](#port-context-path-and-probes)), and that the Aerospike cluster is
reachable from the pod.
