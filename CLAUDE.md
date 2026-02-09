# Claude Code Instructions

## Development Commands

### Deploy Spectre to Kubernetes

Build, push, and deploy spectre to the Kubernetes cluster:

```bash
IMAGE_NAME=ghcr.io/moolen/spectre IMAGE_TAG=test-build make docker-build && \
docker tag docker.io/library/spectre:latest ghcr.io/moolen/spectre:test-build && \
docker push ghcr.io/moolen/spectre:test-build && \
kubectl -n monitoring delete po -l app.kubernetes.io/name=spectre
```

### Local Development (Alternative)

To run spectre locally for development:

```bash
make dev-iterate
```

This command:
1. Stops all running services
2. Rebuilds the spectre binary
3. Starts FalkorDB (graph database)
4. Starts the Spectre server with debug logging

### Stop Development Services

```bash
make dev-stop
```

### View Logs

```bash
make dev-logs
```

Or directly:

```bash
tail -f data-local/logs/spectre.log
```

## Helm Deployment

To deploy via Helm (standard deployment):

```bash
make deploy
```

This uses Helm to deploy to the `monitoring` namespace.

## Build Commands

- `make build` - Build the Go binary
- `make build-ui` - Build the React UI
- `make docker-build` - Build Docker image

## Test Commands

- `make test` - Run all tests
- `make test-go` - Run Go tests only
- `make test-ui` - Run UI tests only
