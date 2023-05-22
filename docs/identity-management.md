
# Identity Management

## Identity types

Cluster API Provider Proxmox (CAPPX) supports multiple methods to provide Proxmox credentials and authorize workload clusters to use them. This guide will go through the different types and provide examples for each. The 3 ways to provide credentials:

* CAPPX Manager bootstrap credentials: The Proxmox username and password provided via `PROXMOX_USERNAME` `PROXMOX_PASSWORD` will be injected into the CAPPX manager binary. These credentials will act as the fallback method should the other two credential methods not be utilized by a workload cluster.
* Credentials via a Secret: Credentials can be provided via a `Secret` that could then be referenced by a `ProxmoxCluster`. This will create a 1:1 relationship between the ProxmoxCluster and Secret and the secret cannot be utilized by other clusters.
* Credentials via a ProxmoxClusterIdentity: `ProxmoxClusterIdentity` is a cluster-scoped resource and enables multiple ProxmoxClusters to share the same set of credentials. The namespaces that are allowed to use the ProxmoxClusterIdentity can also be configured via a `LabelSelector`.

## Examples

### CAPPX Manager Credentials

Setting `PROXMOX_USERNAME` and `PROXMOX_PASSWORD` before initializing the management cluster will ensure the credentials are injected into the manager's binary. More information can be found in the [Cluster API quick start guide](https://cluster-api.sigs.k8s.io/user/quick-start.html)

### Credentials via Secret

Deploy a `Secret` with the credentials in the ProxmoxCluster's namespace:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: secretName
  namespace: <Namespace of ProxmoxCluster>
stringData:
  username: <Username>
  password: <Password>
```

`Note: The secret must reside in the same namespace as the ProxmoxCluster`

Reference the Secret in the ProxmoxCluster Spec:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ProxmoxCluster
metadata:
  name: new-workload-cluster
spec:
  identityRef:
    kind: Secret
    name: secretName
...
```


Once the ProxmoxCluster reconciles, it will set itself as the owner of the Secret and no other ProxmoxClusters will use the same secret. When a cluster is deleted, the secret will also be deleted.

### Credentials via ProxmoxClusterIdentity

Deploy a `Secret` with the credentials in the CAPPX manager namespace (cappx-system by default):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: secretName
  namespace: cappx-system
stringData:
  username: <Username>
  password: <Password>
```

Deploy a `ProxmoxClusterIdentity` that references the secret. The `allowedNamespaces` LabelSelector can also be used to dictate which namespaces are allowed to use the identity. Setting `allowedNamespaces` to nil will block all namespaces from using the identity, while setting it to an empty selector will allow all namespaces to use the identity. The following example uses an empty selector.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ProxmoxClusterIdentity
metadata:
  name: identityName
spec:
  secretName: secretName
  allowedNamespaces:
    selector:
      matchLabels: {}
```

Once the ProxmoxClusterIdentity reconciles, it will set itself as the owner of the Secret and the Secret cannot be used by other identities or ProxmoxClusters. The Secret will also be deleted if the ProxmoxClusterIdentity is deleted.

Reference the ProxmoxClusterIdentity in the ProxmoxCluster.

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ProxmoxCluster
metadata:
  name: new-workload-cluster
spec:
  identityRef:
    kind: ProxmoxClusterIdentity
    name: identityName
...
```

`Note: ProxmoxClusterIdentity cannot be used in conjunction with the WatchNamespace set for the CAPPX manager`
