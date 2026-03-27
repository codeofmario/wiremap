# Development

## Prerequisites

- Go 1.25+
- Node.js 22+
- [air](https://github.com/air-verse/air) — Go hot reload
- Docker — for accessing the Docker daemon

## Getting started

```bash
git clone https://github.com/codeofmario/wiremap.git
cd wiremap
make install   # Install Go and npm dependencies
make dev       # Start backend (air) + frontend (Vite)
```

- **Backend** runs on [http://localhost:7070](http://localhost:7070) with hot reload via `air`
- **Frontend** runs on [http://localhost:5173](http://localhost:5173) with HMR via Vite
- In dev mode, the backend proxies frontend requests to the Vite dev server

## Make targets

| Target | Description |
|--------|-------------|
| `make install` | Install Go and npm dependencies |
| `make dev` | Start backend + frontend with hot reload |
| `make dev-backend` | Start only the Go backend with air |
| `make dev-frontend` | Start only the Vite dev server |
| `make build` | Build production binary with embedded frontend |
| `make run` | Build and run the binary |
| `make wire` | Regenerate Wire dependency injection code |
| `make clean` | Remove build artifacts and reset `web/dist/` |

## Architecture

```
cmd/wiremap/           CLI entry point (Cobra) + Wire DI
internal/wiremap/
  config/              Settings, multi-host YAML parsing
  docker/              Docker client pool (Unix, TCP, TLS, SSH)
  dto/                 Data transfer objects
  errors/              Typed HTTP errors (NotFound, BadRequest, Internal)
  handler/             Gin HTTP handlers
  middleware/          CORS, frontend serving (embed + dev proxy)
  model/               Domain models
  router/              Route registration
  service/             Business logic interfaces + implementations
  ws/                  WebSocket hub (logs, stats, exec)
web/                   Embedded frontend assets (go:embed)
client/                React frontend source
  src/
    components/        Atomic design hierarchy
    hooks/             React hooks (useContainers, useLogs, useStats, ...)
    services/          API and WebSocket clients
    styles/            Global SCSS variables
    types/             TypeScript types
```

### Backend

Layered architecture: `handler -> service -> docker client pool`

- **Gin** for HTTP routing
- **gorilla/websocket** for real-time streaming
- **Docker SDK** for container and network operations
- **Google Wire** for dependency injection
- **Cobra** for CLI

Services define an interface and an unexported struct. Constructors return the interface. Dependencies are injected via constructors, never globals.

Handlers use `apperrors.HandleError(c, err)` for error responses and return DTOs, never raw Docker types.

After changing the dependency graph, regenerate Wire code:

```bash
make wire
```

### Frontend

Atomic design with strict rules:

- **Atoms** (3 files: `.tsx`, `.vm.ts`, `.scss`) — the only level that uses raw HTML, SCSS, and `classnames`
- **Molecules, organisms, templates, pages** (2 files: `.tsx`, `.vm.ts`) — compose atoms only, no raw HTML or CSS

Rules:
- Arrow functions, named exports, no default exports, no `index.ts` barrel files
- Above atoms: no `<div>`, no `className`, no `style={{}}`, no `.scss` imports
- Need a new visual pattern? Create a new atom.

## Building a production binary

```bash
make build
```

This:
1. Builds the React frontend with Vite
2. Copies the built assets to `web/dist/`
3. Compiles the Go binary with `go:embed` to bundle the frontend
4. Outputs `bin/wiremap` — a single static binary with no runtime dependencies

## Docker build

```bash
docker build -t wiremap .
```

The Dockerfile uses a multi-stage build:
1. Node 22 stage builds the frontend
2. Go 1.23 stage builds the backend with embedded frontend
3. Alpine 3.20 final stage with just the binary
