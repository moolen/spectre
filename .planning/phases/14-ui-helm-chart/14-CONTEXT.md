# Phase 14 Context: UI and Helm Chart

## Overview

Phase 14 delivers the UI configuration form for Logz.io integrations and Helm chart support for Kubernetes secret mounting. This completes the v1.2 milestone.

---

## Configuration Form

### Region Selector
- **Type:** Dropdown (not freeform URL)
- **Options:** 5 regions with code + name display
  - `US (United States)`
  - `EU (Europe)`
  - `UK (United Kingdom)`
  - `AU (Australia)`
  - `CA (Canada)`

### Authentication Section
- **Layout:** Separate section from connection settings (not grouped with region)
- **Fields:** Two separate text fields
  - Secret Name (Kubernetes Secret name)
  - Key (key within the Secret containing the API token)
- **Namespace:** Always assumes Spectre's namespace — not user-configurable

### Validation Behavior
- SecretRef existence/validity checked at **connection test time**, not at save
- Users can save untested configurations

### Account Model
- Single Logz.io account per integration instance
- Multiple accounts require creating separate integrations

---

## Connection Test UX

### Loading State
- Test button changes to spinner with loading indicator while testing

### Success Feedback
- Brief toast notification (3-5 seconds)
- Auto-dismisses without user action

### Error Feedback
- **Specific error messages** — show actual failure reason
- Examples:
  - `401 Unauthorized - Invalid API token`
  - `Secret 'my-secret' not found in namespace 'spectre'`
  - `Key 'api-token' not found in Secret 'logzio-creds'`

### Save Behavior
- Save button enabled regardless of test status
- Users can save configurations that haven't been tested

---

## Documentation

### Target Audience
- Platform engineers familiar with Kubernetes concepts
- Assumes knowledge of: Secrets, RBAC, kubectl, Helm

### Secret Example Format
- **Full example** including:
  - YAML manifest for Kubernetes Secret
  - kubectl command to create from literal

### Workflow Documentation
- **High-level steps** for secret rotation
- Not runbook-style (no rollback procedures)
- Example flow: Create new secret → Update SecretRef → Verify

### Troubleshooting
- Not included — errors are self-explanatory for target audience

---

## Helm Chart

### Example Location
- In-line with existing integration config sections
- Not a separate top-level `secrets:` section

### Example Style
- **Commented out** by default
- User uncomments and fills in values to enable

### Pattern Consistency
- Follow existing Helm chart patterns for volumes/mounts
- No new helper templates

### Complexity Level
- Raw volume and volumeMount definitions
- Copy-paste style — no abstractions

---

## Out of Scope

These are explicitly NOT part of Phase 14:
- Secret listing/picker UI (would require additional RBAC)
- Multi-account support in single integration
- Troubleshooting documentation
- Custom namespace selection for secrets
- Helm helper templates for secret mounting

---

*Created: 2026-01-22*
*Source: /gsd:discuss-phase conversation*
