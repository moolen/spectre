Status: Approved

## Summary

Clean up GitHub Actions warnings without changing the release flow semantics.

## Changes

- Replace the release workflow's `github-tag-action` step with an inline shell step that computes the next patch tag, pushes it, and emits the existing outputs.
- Replace `actions/upload-pages-artifact` with explicit tar/gzip packaging plus a direct `actions/upload-artifact` upload for Pages deployment.
- Replace `azure/setup-helm` with explicit Helm binary installation in CI jobs.
- Bump first-party GitHub actions to current major versions that support the Node 24 transition.
- Bump the Docker-maintained GitHub actions in the release workflow to their current major versions.

## Scope

- Update `.github/workflows/release.yml`
- Update `.github/workflows/docs.yml`
- Update `.github/workflows/helm-tests.yml`
- Update `.github/workflows/pr-checks.yml`

## Non-Goals

- Change version bump semantics from the current patch-based release flow.
- Change release artifact contents or chart publishing behavior.
- Change runtime Node versions used by the project itself unless required for the action warning cleanup.
