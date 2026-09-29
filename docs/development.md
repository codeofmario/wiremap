# Development

How to run Wiremap from source, test it, and find your way around the code.

## Prerequisites

- Go 1.25+
- Node.js 22+
- [air](https://github.com/air-verse/air) — Go hot reload
- Docker — for accessing the Docker daemon
- Optional: [kind](https://kind.sigs.k8s.io) and `kubectl` — for a local Kubernetes cluster (see [Kubernetes locally](#kubernetes-locally))

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
| `make test` | Run `go vet`, the Go unit tests and the frontend (Vitest) unit tests |
| `make wire` | Regenerate Wire dependency injection code |
| `make clean` | Remove build artifacts and reset `web/dist/` |

## Architecture

```
cmd/wiremap/           CLI entry point (Cobra) + Wire DI
internal/wiremap/
  config/              Settings, multi-host and cluster YAML parsing
  docker/              Docker client pool (Unix, TCP, TLS, SSH)
  kube/                Kubernetes cluster pool (kubeconfig contexts, in-cluster)
  dto/                 Data transfer objects
  errors/              Typed HTTP errors (NotFound, BadRequest, Forbidden, Internal)
  handler/             Gin HTTP handlers
  middleware/          CORS, frontend serving (embed + dev proxy)
  model/               Domain models
  router/              Route registration
  service/             Business logic interfaces + implementations
  ws/                  WebSocket hubs: Docker (/ws) and Kubernetes (/ws/k8s) logs, stats, exec
web/                   Embedded frontend assets (go:embed)
client/                React frontend source
  src/
    components/        Atomic design hierarchy
    hooks/             React hooks (useContainers, useLogs, useKubeNavigation, useKubeHotkeys, ...)
    services/          API and WebSocket clients, Kubernetes URL routes and `:` commands
    styles/            Global SCSS variables
    types/             TypeScript types
```

### Backend

Layered architecture: `handler -> service -> docker client pool | kube cluster pool`

- **Gin** for HTTP routing
- **gorilla/websocket** for real-time streaming
- **Docker SDK** for container and network operations
- **client-go** for Kubernetes (plus `k8s.io/metrics` for pod metrics)
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

## Testing

Run all checks and tests:

```bash
make test
```

CI (`.github/workflows/ci.yml`) runs the same checks, plus `gofmt` and the race detector, on every push to `main` and on every pull request.

**Backend** — Go's `testing` package, with tests next to the code (`*_test.go`):

- Kubernetes services run against client-go's fake clients (see `service/kube_test_helpers_test.go`), so no cluster is needed.
- Handlers are tested with `httptest` and small fakes of the service interfaces.
- The WebSocket hub is tested with a real test server and WebSocket clients.

**Frontend** — [Vitest](https://vitest.dev), with tests next to the code (`*.test.ts` / `*.test.tsx`):

- Plain logic (URL routes, `:` commands, map layout) runs in Node.
- Hooks use `@testing-library/react`; their files start with `// @vitest-environment jsdom`.
- `npm run test:watch` in `client/` re-runs tests as you edit.

## Kubernetes locally

The demo cluster from the README (*Try it on a local cluster*) is a good cluster to develop against:

```bash
kind create cluster --config examples/kubernetes/kind.yaml
kubectl apply -f examples/kubernetes/demo-app.yaml
kubectl wait --for=condition=Established crd/featureflags.demo.wiremap.io
kubectl apply -f examples/kubernetes/demo-flags.yaml
```

To use it with `make dev`, add it to `wiremap.yml` in the repo root (see [configuration](configuration.md#kubernetes-clusters)):

```yaml
clusters:
  - context: kind-wiremap-demo
```

Or run a built binary with `--kube-context kind-wiremap-demo`.

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
2. Go 1.25 stage builds the backend with embedded frontend
3. Alpine 3.20 final stage with just the binary

## Contributing

1. Create a branch from `main`.
2. Follow the backend and frontend rules above.
3. Add or update tests, then run `make test`.
4. Open a pull request — CI must pass.
