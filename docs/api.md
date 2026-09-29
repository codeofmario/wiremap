# API Reference

The web UI talks to the Wiremap server through a REST API (under `/api`) and a few WebSocket endpoints. You can use them directly too.

- [Docker](#hosts) — hosts, containers, files, networks
- [Kubernetes](#kubernetes) — clusters, the map, objects, actions
- [WebSockets](#websocket-endpoints) — logs, stats, shells and live map updates

Errors come back as `{ "message": "..." }` with a matching HTTP status (`400`, `403`, `404`, `500`).

## REST Endpoints

### Hosts

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/hosts` | List configured Docker hosts |

**Response:**

```json
[
  { "name": "local", "connected": true }
]
```

### Containers

All container endpoints accept an optional `?host=` query parameter to select the Docker host.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/containers` | List all containers |
| `GET` | `/api/containers/:id` | Inspect a container |
| `PUT` | `/api/containers/:id/env` | Update environment variables |

**List containers response:**

```json
[
  {
    "id": "abc123...",
    "names": ["/my-app"],
    "image": "nginx:latest",
    "state": "running",
    "status": "Up 2 hours",
    "created": 1735689600,
    "ports": [{ "privatePort": 80, "publicPort": 8080, "type": "tcp" }],
    "labels": { "com.docker.compose.service": "web" },
    "networks": {
      "bridge": { "ipAddress": "172.17.0.2", "gateway": "172.17.0.1", "macAddress": "02:42:ac:11:00:02", "networkId": "net123...", "aliases": null }
    }
  }
]
```

**Update environment variables:**

```bash
PUT /api/containers/:id/env?host=local
Content-Type: application/json

{
  "env": ["NODE_ENV=production", "PORT=3000"]
}
```

This recreates the container with the new environment variables.

### Filesystem

Browse and edit files inside a running container.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/containers/:id/fs` | List directory contents |
| `GET` | `/api/containers/:id/fs/read` | Read file content (max 512 KB) |
| `PUT` | `/api/containers/:id/fs/write` | Write file content |

**List directory:**

```bash
GET /api/containers/:id/fs?path=/etc&host=local
```

```json
[
  { "name": "nginx.conf", "path": "/etc/nginx.conf", "isDir": false, "size": 1234, "permissions": "-rw-r--r--" },
  { "name": "conf.d", "path": "/etc/conf.d", "isDir": true, "size": 4096, "permissions": "drwxr-xr-x" }
]
```

**Read file:**

```bash
GET /api/containers/:id/fs/read?path=/etc/nginx.conf&host=local
```

```json
{
  "path": "/etc/nginx.conf",
  "content": "worker_processes auto;\n...",
  "binary": false,
  "size": 1234
}
```

**Write file:**

```bash
PUT /api/containers/:id/fs/write?host=local
Content-Type: application/json

{
  "path": "/etc/nginx.conf",
  "content": "worker_processes 4;\n..."
}
```

### Networks

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/networks` | List all networks |
| `GET` | `/api/networks/:id` | Inspect a network |

**Inspect network response:**

```json
{
  "id": "net123...",
  "name": "my-network",
  "driver": "bridge",
  "scope": "local",
  "containers": [
    { "id": "abc123...", "name": "my-app", "ipv4Address": "172.18.0.2/16" }
  ]
}
```

### Kubernetes

Every Kubernetes endpoint works on one configured cluster, named in the path as `:cluster` (see [configuration](configuration.md#kubernetes-clusters)).

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/k8s/clusters` | List configured clusters |
| `GET` | `/api/k8s/clusters/:cluster/topology` | One level of the map as a graph (see below) |
| `GET` | `/api/k8s/clusters/:cluster/kinds` | Every listable kind the cluster serves, including CRDs |
| `GET` | `/api/k8s/clusters/:cluster/objects?group=&version=&resource=&namespace=&continue=` | Objects of any kind, 500 per page |
| `GET` | `/api/k8s/clusters/:cluster/counts?namespace=&kinds=/Pod,apps/Deployment` | Object counts per `group/Kind` in a namespace (empty for all); cluster-scoped kinds are counted cluster-wide, kinds the cluster doesn't serve are left out |
| `GET` | `/api/k8s/clusters/:cluster/resources/:kind/:name?group=&namespace=` | Any object as YAML plus its recent events |
| `GET` | `/api/k8s/clusters/:cluster/namespaces/:namespace/pods/:name` | Pod summary with container states |

`group` is the kind's API group (`apps`, `argoproj.io`, …); leave it empty for core kinds like Pod or Service.

`kinds` also returns each kind's `shortNames` (e.g. `po`, `deploy`), and each item from `objects` has a `drillable` flag: whether it can be opened at the `object` topology level.

**List clusters response:**

```json
[
  { "name": "prod", "connected": true }
]
```

**Topology levels** — pick one with the `level` query parameter:

| `level` | Extra params | Nodes returned |
|---------|--------------|----------------|
| `cluster` (default) | `lens=namespaces` (default), `nodes`, `storage` or `access` | Namespaces sized by pod count; cluster nodes; PVC → PersistentVolume → StorageClass; or subject (User/Group/ServiceAccount) → binding → role for every non-`system:` binding |
| `namespace` | `namespace` | See below |
| `object` | `namespace`, `kind`, `group`, `name` | One object and what it contains, see below |
| `node` | `name` | Pods scheduled on the node, grouped by namespace |

At namespace level the graph contains:

- **Traffic:** Ingresses, Gateway API Gateways and HTTP/GRPC/TLS/TCP/UDP routes (when the CRDs are installed), Services — including selector-less Services, linked through their EndpointSlices
- **Workloads:** Deployments, StatefulSets, DaemonSets, CronJobs, Jobs, ReplicationControllers, bare Pods, and any other object that owns pods (bare ReplicaSets, Argo Rollouts, operators' custom resources)
- **Scaling & policy:** HorizontalPodAutoscalers, PodDisruptionBudgets, NetworkPolicies, ResourceQuotas, LimitRanges
- **Config & storage:** ConfigMaps, Secrets (including image pull secrets), PVCs (including StatefulSet volume claim templates), their PersistentVolumes and StorageClasses
- **Access:** ServiceAccounts workloads run as, and the RoleBindings/ClusterRoleBindings and Roles/ClusterRoles granting them permissions

At object level, the graph shows the object and what's directly inside it — one level at a time:

| Kind | Shows |
|------|-------|
| Deployment, StatefulSet, DaemonSet, ReplicaSet, Job, custom controllers | Its Pods, grouped by node |
| CronJob | Its Jobs |
| Pod | Its init and app containers (kind `Container`) |
| Service | The Pods it sends traffic to, grouped by node |
| Ingress | The Services it routes to |
| ConfigMap, Secret, PersistentVolumeClaim | The Pods using it; for a claim, also its PersistentVolume and StorageClass |

Nodes with `drillable: true` can be opened as their own object level.

```bash
GET /api/k8s/clusters/prod/topology?level=object&namespace=shop&group=apps&kind=Deployment&name=api
```

```json
{
  "level": "object",
  "nodes": [
    { "id": "Deployment/shop/api", "kind": "Deployment", "apiGroup": "apps", "name": "api", "namespace": "shop",
      "status": "warning", "summary": "1/2 ready", "drillable": false, "weight": 2 },
    { "id": "Pod/shop/api-5d8f7-by", "kind": "Pod", "name": "api-5d8f7-by", "namespace": "shop",
      "status": "error", "summary": "CrashLoopBackOff · 0/1 · 7 restarts", "drillable": true,
      "group": "Node//node-b", "weight": 1, "details": { "Node": "node-b", "Pod IP": "10.0.0.12" } }
  ],
  "edges": [
    { "source": "Deployment/shop/api", "target": "Pod/shop/api-5d8f7-by", "kind": "owns" }
  ],
  "groups": [
    { "id": "Node//node-b", "name": "node-b", "kind": "Node" }
  ]
}
```

`status` is one of `healthy`, `warning`, `error`, `idle`. A node's `group` is the box it's drawn in on the map (a node or namespace); `apiGroup` is its kind's API group.

| Edge `kind` | Meaning |
|-------------|---------|
| `routes` | Ingress/Gateway/route → Service |
| `selects` | Service → workload or Pod |
| `owns` | Owner → ReplicaSet/Job/Pod, CronJob → Job, Pod → Container |
| `uses` | Workload/Pod → ConfigMap/Secret/PVC, PV → StorageClass |
| `bound` | PVC → PersistentVolume |
| `scales` | HorizontalPodAutoscaler → workload |
| `protects` | PodDisruptionBudget → workload |
| `applies` | NetworkPolicy → workload it selects |
| `allows` | Workload → workload a NetworkPolicy lets it reach |
| `runs-as` | Workload/Pod → ServiceAccount |
| `binds` | (Cluster)RoleBinding → ServiceAccount |
| `grants` | (Cluster)RoleBinding → (Cluster)Role |

How `status` is worked out: built-in kinds have their own rules (e.g. ready replicas, pod phase and restarts). Everything else, including custom resources, uses `status.phase` or the `Ready`/`Available`/`Accepted`/`Programmed` conditions.

The resource response also carries `actions`, the controls the UI offers for the object: `edit`, `delete`, `restart`, `trigger`, and the current `replicas`, `suspended` (CronJobs) or `unschedulable` (Nodes) when they apply.

**Object list response:** `{ "items": [...], "continue": "<token>" }` — pass `continue` back to get the next page; it's absent on the last page.

Secret values are never returned: each key's value shows as `<redacted>`. If the cluster's RBAC denies a request, you get `403` with Kubernetes' message — except for optional parts of the namespace map (RBAC, policies, storage), which are just left out.

### Kubernetes actions

Actions run as Wiremap's cluster user, so the cluster's RBAC decides what's allowed. All of them take `?group=&namespace=` like the resource endpoint.

| Method | Endpoint | Body | Effect |
|--------|----------|------|--------|
| `PUT` | `/api/k8s/clusters/:cluster/resources/:kind/:name` | `{ "yaml": "..." }` | Replace the object with the edited manifest. Its `resourceVersion` makes concurrent edits fail with a conflict; kind, name and namespace can't change |
| `DELETE` | `/api/k8s/clusters/:cluster/resources/:kind/:name` | | Delete (background propagation) |
| `POST` | `.../resources/:kind/:name/scale` | `{ "replicas": 3 }` | Scale through the scale subresource (works for CRDs that support it) |
| `POST` | `.../resources/:kind/:name/restart` | | Rollout restart of a Deployment, StatefulSet or DaemonSet |
| `POST` | `.../resources/:kind/:name/trigger` | | Start a Job from a CronJob now; returns `{ "job": "<name>" }` |
| `POST` | `.../resources/:kind/:name/suspend` | `{ "value": true }` | Suspend or resume a CronJob |
| `POST` | `/api/k8s/clusters/:cluster/nodes/:name/cordon` | `{ "value": true }` | Cordon or uncordon a node |
| `POST` | `/api/k8s/clusters/:cluster/nodes/:name/drain` | | Cordon, then evict all pods except DaemonSet and static pods, respecting PodDisruptionBudgets; returns `{ "refused": [...] }` for evictions that were blocked |

**Editing Secrets:** the manifest comes back with values shown as `<redacted>`. Leaving a value as `<redacted>` keeps the stored one, writing a new value replaces it, and removing a key deletes it.

Requests that change something (`POST`, `PUT`, `DELETE`) from another website are rejected with `403`: the `Origin` header must match the Wiremap host. Clients that send no `Origin`, like curl or scripts, are allowed.

## WebSocket Endpoints

### Log and Stats Streaming

**Endpoint:** `ws://host:port/ws`

Subscribe and unsubscribe to log and stats streams by sending JSON messages.

#### Log streaming

**Subscribe:**

```json
{ "event": "logs:subscribe", "data": { "containerId": "abc123", "hostId": "local" } }
```

**Server sends log entries:**

```json
{ "event": "logs:data", "data": { "containerId": "abc123", "type": "stdout", "text": "2025-01-01T00:00:00Z INFO starting server" } }
```

**Unsubscribe:**

```json
{ "event": "logs:unsubscribe", "data": { "containerId": "abc123" } }
```

**Error:**

```json
{ "event": "logs:error", "data": { "containerId": "abc123", "message": "container not found" } }
```

#### Stats streaming

**Subscribe:**

```json
{ "event": "stats:subscribe", "data": { "containerId": "abc123", "hostId": "local" } }
```

**Server sends stats snapshots:**

```json
{
  "event": "stats:data",
  "data": {
    "containerId": "abc123",
    "stats": {
      "cpuPercent": 12.5,
      "memoryUsage": 52428800,
      "memoryLimit": 1073741824,
      "memoryPercent": 4.88,
      "networkRx": 1048576,
      "networkTx": 524288,
      "timestamp": "2025-01-01T00:00:00Z"
    }
  }
}
```

**Unsubscribe:**

```json
{ "event": "stats:unsubscribe", "data": { "containerId": "abc123" } }
```

### Interactive Shell

**Endpoint:** `ws://host:port/ws/exec/:id?host=local`

Binary WebSocket for interactive container shell I/O. Supports TTY resize:

```json
{ "event": "resize", "cols": 120, "rows": 40 }
```

All other messages are raw binary data streamed between the terminal and the container process.

### Kubernetes pod streams

**Endpoint:** `ws://host:port/ws/k8s`

Every message identifies a pod container with `cluster`, `namespace`, `pod` and `container`. Server events carry a `key` of the form `cluster/namespace/pod/container` (as `containerId` for log entries).

| Client event | Server events |
|--------------|---------------|
| `pod-logs:subscribe` / `pod-logs:unsubscribe` | `pod-logs:data`, `pod-logs:error` |
| `pod-stats:subscribe` / `pod-stats:unsubscribe` | `pod-stats:data` (every 5s, same `stats` shape as Docker, without network counters), `pod-stats:error` |
| `topology:watch` / `topology:unwatch` | `topology:changed` when objects shown at that scope change (debounced to 1s), `topology:error` |

`topology:watch` takes `{ "cluster": "prod", "namespace": "shop" }`; an empty namespace watches the cluster-level lenses. Its events carry `key` = `cluster/namespace`. The canvas refetches the topology on each `topology:changed`.

```json
{ "event": "pod-logs:subscribe", "data": { "cluster": "prod", "namespace": "shop", "pod": "api-5d8f7-by", "container": "app" } }
```

Pod metrics come from [metrics-server](https://github.com/kubernetes-sigs/metrics-server). Without a CPU limit, `cpuPercent` is relative to one core; without a memory limit, `memoryLimit` is the node's allocatable memory.

### Kubernetes pod shell

**Endpoint:** `ws://host:port/ws/k8s/exec/:cluster/:namespace/:pod?container=app`

Same protocol as the Docker shell: binary frames for terminal I/O, text `resize` events for the TTY size. Starts `bash` when the image has it, otherwise `sh`.
