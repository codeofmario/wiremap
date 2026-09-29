# Configuration

You can configure Wiremap with command-line flags, a `wiremap.yml` file, or both.

- [Flags](#flags)
- [Config file](#config-file)
- [Docker hosts](#docker-hosts)
- [Kubernetes clusters](#kubernetes-clusters)
- [Permissions](#permissions)
- [Which setting wins](#which-setting-wins)
- [Running Wiremap in Docker](#running-wiremap-in-docker)

## Flags

| Flag | Default | What it does |
|------|---------|--------------|
| `-p`, `--port` | `7070` | Port for the web UI |
| `--host` | | Docker host to connect to. Repeat it for several hosts |
| `--config` | | Path to a config file |
| `--kubeconfig` | | Kubeconfig file. Turns on Kubernetes support |
| `--kube-context` | | Kubeconfig context to connect to. Repeat it for several clusters |
| `--dev` | `false` | Development only: serve the frontend from Vite on `:5173` |

```bash
# Two Docker hosts
wiremap --host unix:///var/run/docker.sock --host tcp://prod:2375

# A config file, on another port
wiremap --config wiremap.yml -p 9090
```

## Config file

Wiremap reads `wiremap.yml` from the folder you start it in. To use another file, pass `--config /path/to/wiremap.yml`.

A complete example:

```yaml
hosts:
  - name: local
    url: unix:///var/run/docker.sock

  - name: production
    url: tcp://10.0.1.5:2376
    tls:
      cert: /path/to/cert.pem
      key: /path/to/key.pem
      ca: /path/to/ca.pem

  - name: staging
    url: ssh://deploy@staging.example.com

clusters:
  - name: prod
    kubeconfig: ~/.kube/config
    context: prod-eu
```

## Docker hosts

Each host needs a `name` (shown in the top bar) and a `url`.

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Name shown in the UI |
| `url` | Yes | How to reach the Docker daemon (see below) |
| `tls.cert`, `tls.key`, `tls.ca` | No | Client certificate, key and CA for TLS over TCP |

**Local socket** — the default when nothing is configured:

```yaml
- name: local
  url: unix:///var/run/docker.sock
```

**TCP**, unencrypted:

```yaml
- name: remote
  url: tcp://192.168.1.100:2375
```

**TCP with TLS:**

```yaml
- name: secure-remote
  url: tcp://192.168.1.100:2376
  tls:
    cert: ~/.docker/cert.pem
    key: ~/.docker/key.pem
    ca: ~/.docker/ca.pem
```

**SSH:**

```yaml
- name: via-ssh
  url: ssh://deploy@server.example.com
```

SSH uses your SSH agent and `~/.ssh/config`. The remote user must be allowed to use Docker there.

## Kubernetes clusters

Kubernetes is off until you add at least one cluster. Docker hosts and clusters work side by side; switch between them in the top bar.

With flags:

```bash
wiremap --kubeconfig ~/.kube/config                    # the kubeconfig's current context
wiremap --kube-context prod-eu --kube-context staging  # several contexts from the default kubeconfig
```

Or in `wiremap.yml`:

```yaml
clusters:
  - name: prod
    kubeconfig: ~/.kube/config
    context: prod-eu

  - name: staging
    context: staging      # uses $KUBECONFIG or ~/.kube/config

  - name: in-cluster
    inCluster: true       # when Wiremap runs as a pod in the cluster
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | No | Name shown in the UI. Defaults to the context name |
| `kubeconfig` | No | Kubeconfig file. Defaults to `$KUBECONFIG`, then `~/.kube/config` |
| `context` | No | Context to use. Defaults to the kubeconfig's current context |
| `inCluster` | No | Use the pod's service account instead of a kubeconfig |

Some kubeconfigs log in through a helper program, like `aws eks get-token` or `gke-gcloud-auth-plugin`. That program must be installed where Wiremap runs. The Docker image doesn't include them, so use a token or certificate kubeconfig there.

## Permissions

Wiremap acts as the user in your kubeconfig (or the pod's service account). So that user's Kubernetes permissions (RBAC) decide what you can do:

- Things it can't read are simply left off the map.
- Actions it can't do show the cluster's "forbidden" error.

**Full access** — see everything and use every action:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: wiremap
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: [get, list, watch, create, update, patch, delete]
```

**Read-only** — look around, read logs and metrics, change nothing:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: wiremap-readonly
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: [get, list, watch]
```

To keep the pod shell with the read-only role, also allow `create` and `get` on `pods/exec`.

`watch` is what makes the map update live. Without it, the map refreshes every 30 seconds.

**Security:** Wiremap has no login. Anyone who can open its port gets these permissions. Secret values are never sent to the browser, and requests from other websites are blocked — but still keep Wiremap on a trusted network, and prefer the read-only role if others can reach it.

## Which setting wins

**Docker hosts** — the first match wins:

1. Hosts in the file given with `--config`
2. `--host` flags
3. Hosts in `wiremap.yml` in the current folder
4. The local socket, `unix:///var/run/docker.sock`

**Kubernetes clusters** — `--kube-context` or `--kubeconfig` flags replace the `clusters` in the config file. With neither, Kubernetes stays off.

## Running Wiremap in Docker

Mount the Docker socket read-only:

```bash
docker run -d -p 7070:7070 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  codeofmario/wiremap
```

For remote hosts or Kubernetes, also mount your config file, certificates and kubeconfig:

```bash
docker run -d -p 7070:7070 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v ./wiremap.yml:/etc/wiremap/wiremap.yml:ro \
  -v ./certs:/certs:ro \
  -v ~/.kube/config:/etc/wiremap/kubeconfig:ro \
  codeofmario/wiremap --config /etc/wiremap/wiremap.yml
```

Inside that `wiremap.yml`, use the paths as seen by the container — for example `kubeconfig: /etc/wiremap/kubeconfig` and `cert: /certs/cert.pem`.
