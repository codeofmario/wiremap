export PATH := $(HOME)/sdk/go1.25.8/bin:$(HOME)/go/bin:$(HOME)/gopath/bin:$(PATH)
export GOPATH := $(HOME)/gopath

.PHONY: dev dev-backend dev-frontend build build-frontend build-backend wire install clean

# Development - run both backend and frontend with hot reload
dev: install
	@echo "Starting dev mode..."
	@make -j2 dev-backend dev-frontend

dev-backend: wire
	@echo "Starting Go backend with air..."
	air

dev-frontend:
	@echo "Starting Vite dev server..."
	cd client && npm run dev

# Build production binary with embedded frontend
build: build-frontend wire build-backend

build-frontend:
	cd client && npm run build
	rm -rf web/dist
	cp -r client/dist web/dist

build-backend:
	go build -o bin/wiremap ./cmd/wiremap

# Wire dependency injection
wire:
	cd cmd/wiremap && wire

# Install all dependencies
install:
	cd client && npm install
	go mod tidy

# Clean build artifacts
clean:
	rm -rf bin/ web/dist client/dist
	mkdir -p web/dist
	echo '<!DOCTYPE html><html><body>Run make build</body></html>' > web/dist/index.html

# Run the built binary
run: build
	./bin/wiremap
