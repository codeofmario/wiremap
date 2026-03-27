## Build frontend
FROM node:22-alpine AS frontend
WORKDIR /app/client
COPY client/package.json client/package-lock.json* ./
RUN npm ci
COPY client/ .
RUN npm run build

## Build Go binary
FROM golang:1.23-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/client/dist ./web/dist
RUN CGO_ENABLED=0 go build -o wiremap ./cmd/wiremap

## Final image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && \
    addgroup -g 1000 wiremap && \
    adduser -D -u 1000 -G wiremap wiremap
COPY --from=backend /app/wiremap /usr/local/bin/wiremap
USER wiremap
EXPOSE 7070
ENTRYPOINT ["wiremap"]
