# CI/CD Pipeline Documentation

This project uses GitHub Actions for continuous integration and deployment.

## Workflows

### PR Checks (`pr-checks.yml`)
Runs on every pull request to `master` branch.

**Steps:**
1. Runs all Go tests (unit + integration)
2. Builds Docker image with tag format: `ghcr.io/moolen/spectre:pr-{PR_NUMBER}-{COMMIT_SHA_SHORT}`
3. Artifacts are uploaded for review (not pushed to registry)

**Triggers:** Pull requests to `master`

### Release (`release.yml`)
Runs on every push to `master` branch (after merge).

**Steps:**
1. Builds Docker container image
2. Pushes to `ghcr.io/moolen/spectre` with tags:
   - `master` (branch tag)
   - `master-{COMMIT_SHA}` (commit tag)
   - Semver tags (if releases are tagged)
3. Packages and pushes Helm chart to `ghcr.io/moolen/charts` as OCI artifact

**Triggers:** Push to `master`

## Required GitHub Secrets

No additional secrets are required. The pipeline uses `GITHUB_TOKEN` which is automatically available in GitHub Actions.

### Permissions

The workflows require:
- `contents: read` - To checkout the repository
- `packages: write` - To push container images and Helm charts to ghcr.io

## Container Registry

All images are pushed to:
- Container images: `ghcr.io/moolen/spectre`
- Helm charts: `ghcr.io/moolen/charts`

Make sure your GitHub repository is configured to allow publishing packages.

## Building Locally

To test the build locally before pushing:

```bash
# Build Go binary and UI
make build
make build-ui

# Build Docker image
make docker-build

# Run tests
make test
make test-integration
```

## Helm Chart Deployment

To deploy using the pushed Helm chart:

```bash
# Add the Helm chart repository
helm repo add spectre oci://ghcr.io/moolen/charts

# Install the chart
helm install spectre spectre/spectre --namespace monitoring --create-namespace
```

## Troubleshooting

### Image not pushing to ghcr.io
1. Verify `GITHUB_TOKEN` has `packages: write` permission
2. Check that the repository visibility allows package publishing
3. Ensure the GitHub Actions workflow file has `packages: write` in permissions

### Helm chart push fails
1. Make sure `chart/Chart.yaml` exists and has a `version` field
2. Verify the Helm setup-action is using compatible version
3. Check registry login credentials

## Environment Variables

The workflows inherit the following from your Makefile:
- `BINARY_NAME=spectre`
- `IMAGE_NAME=spectre` (overridden to use ghcr.io in workflows)
- `CHART_PATH=./chart`
