Status: Approved

## Summary

Clean up GitHub Actions warnings without changing the release flow semantics.

## Changes

- Remove the unsupported `initial_version` input from the release workflow's `github-tag-action` step.
- Set `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true` at the workflow level for workflows that currently emit Node 20 deprecation warnings.
- Bump first-party GitHub actions to current major versions that support the Node 24 transition.
- Keep `actions/upload-pages-artifact@v3` because it is still the current major release for that action and is documented as compatible with newer `deploy-pages` releases.

## Scope

- Update `.github/workflows/release.yml`
- Update `.github/workflows/docs.yml`
- Update `.github/workflows/helm-tests.yml`
- Update `.github/workflows/pr-checks.yml`

## Non-Goals

- Rewrite semantic version tagging logic.
- Change release artifact contents or chart publishing behavior.
- Change runtime Node versions used by the project itself unless required for the action warning cleanup.
