# Aerospike Backup Service

This Helm chart allows you to configure and run our official [Aerospike Backup Service](https://github.com/aerospike/aerospike-backup-service)
docker image on a Kubernetes cluster.

This helm chart sets up a `Deployment` for Aerospike Backup Service.


## Prerequisites
- Kubernetes cluster
- Helm v3
- An Aerospike cluster that can connect to Pods in the Kubernetes cluster.
  The Aerospike cluster can be deployed in the same Kubernetes cluster using [Aerospike Kubernetes Operator](https://docs.aerospike.com/cloud/kubernetes/operator)

## Adding the helm chart repository

Add the `aerospike` helm repository if not already done

```shell
helm repo add aerospike https://aerospike.github.io/helm-charts
```

## Supported configuration

### Configuration

| Parameter                 | Description                                                                                                                                                                          | Default                        |
|---------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------|
| `replicaCount`            | Configures the number Aerospike Backup Service instance pods to run.                                                                                                                 | '1'                            |
| `image`                   | Configures Aerospike Aerospike Backup Service image repository, tag and pull policy.                                                                                                 | see [values.yaml](values.yaml) |
| `imagePullSecrets`        | For Private docker registries, when authentication is needed.                                                                                                                        | see [values.yaml](values.yaml) |
| `args`                    | Arguments to pass to ABS                                                                                                                                                             | `[]`                           |
| `backupServiceConfig`     | ABS cluster configuration deployed to `/etc/aerospike-backup-service/aerospike-backup-service.yml`.                                                                                  | see [values.yaml](values.yaml) |
| `initContainers`          | List of initContainers added to each abs pods for custom cluster behavior.                                                                                                           | `[]`                           |
| `serviceAccount`          | Service Account details like name and annotations.                                                                                                                                   | see [values.yaml](values.yaml) |
| `podAnnotations`          | Additional pod [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/). Should be specified as a map of annotation names to annotation values. | `{}`                           |
| `podLabels`               | Additional pod [labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/). Should be specified as a map of label names to label values.                     | `{}`                           |
| `podSecurityContext`      | Pod [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)                                                                                   | `{}`                           |
| `securityContext`         | Container [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-container)                                    | `{}`                           |
| `service`                 | Load-Balancer configuration for more details please refer to a Load-Balancer docs.                                                                                                   | `{}`                           |
| `resources`               | Resource [requests and limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) for the abs pods.                                                     | `{}`                           |
| `autoscaling`             | Enable the horizontal pod auto-scaler.                                                                                                                                               | see [values.yaml](values.yaml) |
| `volumes`                 | List of additional volumes to attach to the abs pod.                                                                                                                                 | see [values.yaml](values.yaml) |
| `extraVolumeMounts`       | Extra volume mounts corresponding to the volumes added to `extraVolumes`.                                                                                                            | see [values.yaml](values.yaml) |
| `extraSecretVolumeMounts` | Extra secret volume mounts corresponding to the volumes added to `extraVolumes`.                                                                                                     | see [values.yaml](values.yaml) |
| `affinity`                | [Affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity)  rules if any for the pods.                                          | `{}`                           |
| `nodeSelector`            | [Node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector)  for the pods.                                                                | `{}`                           |
| `tolerations`             | [Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)  for the pods.                                                                          | `{}`                           |



## Deploy the ABS

We recommend creating a new `.yaml` for providing configuration values to the helm chart for deployment.
See the [examples](examples) folder for examples.


We recommend naming the file with the name of the ABS service. For example if you want to name your service as
`abs`, create a file `abs-values.yaml`.
Once you have created this custom values file, deploy the ABS, using the following commands.

### Create a new namespace
We recommend using `aerospike` namespace for the ABS. If the namespace does not exist run the following command:
```shell
kubectl create namespace aerospike
```

### Create secrets

You can create additional secrets, for confidential data like TLS certificates, S3 credentials, and Aerospike cluster password these are mounted to the ABS pods as files.
The ABS can then be configured to use these secrets. 

### Deploy Aerospike Backup Service

```shell
# helm install --namespace <target namespace> <helm release name/cluster name> -f <path to custom values yaml> aerospike/aerospike-backup-service
helm install abs "aerospike/aerospike-backup-service" \
--namespace aerospike \
--values abs-values.yaml \
--create-namespace \
--wait
```

Here `abs` is the release name for the Aerospike Backup Service.

On successful deployment you should see output similar to below:

```shell
NAME: abs
LAST DEPLOYED: Wed May 22 11:46:26 2024
NAMESPACE: aerospike
STATUS: deployed
REVISION: 1
NOTES:
```

## List pods for the ABS
To list the pods for the ABS run the following command:
```shell
# kubectl get pods --namespace aerospike --selector=app=<helm release name>-aerospike-backup-service
kubectl get pods --namespace aerospike --selector=app=abs-aerospike-backup-service
```

You should see output similar to the following:
```shell
NAME                                            READY   STATUS    RESTARTS   AGE
abs-aerospike-backup-service-544cc664c9-g5gdj   1/1     Running   0          22m
```

## Get logs for all ABS instances

```shell
# kubectl -n aerospike logs -f deployment/<helm release name>-aerospike-backup-service
# Skip the -f flag to get a one time dump of the log
kubectl -n aerospike logs -f deployment/abs-aerospike-backup-service
```

## Get logs for one ABS pod

```shell
# kubectl -n aerospike logs -f <helm release name>-aerospike-backup-service-<random-hash>
# Skip the -f flag to get a one time dump of the log
kubectl -n aerospike logs -f abs-aerospike-backup-service-544cc664c9-g5gdj
```

## Updating ABS configuration

Edit the `backupServiceConfig` section in the custom values file and save the changes.

Upgrade the ABS deployment using the following command.

```shell
#helm upgrade --namespace <target namespace> <helm release name> -f <path to custom values yaml file> aerospike/aerospike-backup-service
helm upgrade --namespace aerospike abs -f abs-values.yaml aerospike/aerospike-backup-service
```

On successful execution of the command the ABS pods will undergo a rolling restart and come up with the new configuration.

To verify the changes are applied
- [List the pods](#list-pods-for-the-abs)
- [Verify the configuration in abs logs](#get-logs-for-all-abs-instances)

**_NOTE:_** The changes might take some time to apply. If you do not see the desired ABS config try again after some time.
If ABS pods are not being listed or report status as crashed see [troubleshooting](#troubleshooting).

## Scaling up/down the ABS

Edit the `replicaCount` to the desired ABS count and upgrade the ABS deployment using the following command.

```shell
#helm upgrade --namespace <target namespace> <helm release name> -f <path to custom values yaml file> aerospike/aerospike-backup-service
helm upgrade --namespace aerospike abs -f abs-values.yaml aerospike/aerospike-backup-service
```

Verify that the ABS have been scaled.
- [List the pods](#list-pods-for-the-abs) and verify the count of abs instances is as desired

**_NOTE:_** The changes might take some time to apply. If you do not see the desired count try again after some time.
If ABS pods are not being listed or report status as crashed see [troubleshooting](#troubleshooting).

## Troubleshooting

### ABS pods not listed

Check for any error events on the Deployment created for the ABS.
```shell
# kubectl -n aerospike describe deployment <helm release name>-aerospike-backup-service
kubectl -n aerospike describe deployment abs-aerospike-backup-service
```

### ABS pods stuck in `init` or `pending` state

Check for any error events on the pod created for the ABS.
```shell
# kubectl -n aerospike describe pod <helm release name>-aerospike-backup-service--<random-hash>
kubectl -n aerospike describe pod abs-aerospike-backup-service-544cc664c9-g5gdj
```

The most likely reason is secret listed in `backupServiceConfig` has not been created in the ABS namespace.

### ABS pods in crashed state

The most likely reason is ABS configuration provided in `backupServiceConfig` is invalid.
Verify this by [viewing the abs logs](#get-logs-for-all-abs-instances), fix and [update](#updating-abs-configuration)
the ABS configuration.
