# API Reference

Wiremap exposes a REST API and WebSocket endpoints. All REST endpoints are prefixed with `/api`.

## REST Endpoints

### Hosts

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/hosts` | List configured Docker hosts |

**Response:**

```json
[
  { "id": "local", "name": "local", "url": "unix:///var/run/docker.sock" }
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
    "ports": [{ "privatePort": 80, "publicPort": 8080, "type": "tcp" }],
    "networks": ["bridge"],
    "labels": { "com.docker.compose.service": "web" }
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
