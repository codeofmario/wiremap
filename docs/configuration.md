# Configuration

Wiremap can be configured via CLI flags, a YAML config file, or both. CLI flags take precedence over the config file.

## Config file

By default, Wiremap looks for `wiremap.yml` in the current working directory. You can specify a different path with `--config`:

```bash
wiremap --config /etc/wiremap/wiremap.yml
```

### Full example

```yaml
hosts:
  - name: local
    url: unix:///var/run/docker.sock

  - name: production
    url: tcp://10.0.1.5:2375
    tls:
      cert: /path/to/cert.pem
      key: /path/to/key.pem
      ca: /path/to/ca.pem

  - name: staging
    url: ssh://deploy@staging.example.com
```

## Host configuration

Each host entry requires a `name` and `url`. The `tls` block is optional and only applies to TCP connections.

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Display name shown in the host selector |
| `url` | Yes | Docker daemon connection URL |
| `tls.cert` | No | Path to client certificate PEM file |
| `tls.key` | No | Path to client key PEM file |
| `tls.ca` | No | Path to CA certificate PEM file |

### Connection types

**Unix socket** (default when no config is provided):

```yaml
- name: local
  url: unix:///var/run/docker.sock
```

**TCP** (unencrypted):

```yaml
- name: remote
  url: tcp://192.168.1.100:2375
```

**TCP with TLS**:

```yaml
- name: secure-remote
  url: tcp://192.168.1.100:2376
  tls:
    cert: ~/.docker/cert.pem
    key: ~/.docker/key.pem
    ca: ~/.docker/ca.pem
```

**SSH**:

```yaml
- name: via-ssh
  url: ssh://deploy@server.example.com
```

SSH connections use your local SSH agent and config (`~/.ssh/config`). Make sure the target host has Docker installed and the SSH user has access to the Docker socket.

## Precedence

Configuration is resolved in this order (first match wins):

1. `--config` flag pointing to a YAML file
2. `--host` CLI flags (can be repeated)
3. `wiremap.yml` in the current working directory
4. Default: `unix:///var/run/docker.sock`

## CLI flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--port` | `-p` | `7070` | Port to listen on |
| `--host` | | | Docker host URL (can be repeated) |
| `--config` | | | Path to config YAML file |
| `--dev` | | `false` | Dev mode: proxy frontend requests to Vite at `:5173` |

### Examples

```bash
# Connect to two hosts via CLI flags
wiremap --host unix:///var/run/docker.sock --host tcp://prod:2375

# Use a config file on a custom port
wiremap --config wiremap.yml -p 9090
```

## Docker deployment

When running Wiremap in Docker, mount the Docker socket read-only:

```bash
docker run -d \
  -p 7070:7070 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  codeofmario/wiremap
```

To connect to remote hosts from a Docker container, mount your config and TLS certs:

```bash
docker run -d \
  -p 7070:7070 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v ./wiremap.yml:/etc/wiremap/wiremap.yml:ro \
  -v ./certs:/certs:ro \
  codeofmario/wiremap --config /etc/wiremap/wiremap.yml
```
