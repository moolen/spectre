---
title: Storage Settings
description: Configure embedded storage and ingest-time data scrubbing
keywords: [storage, configuration, scrub, security]
---

# Storage Settings

## Sensitive Data Scrubbing

**Flag:** `--scrub-sensitive-data`
**Type:** Boolean
**Default:** `false`

**Purpose:** Scrub sensitive values before newly ingested resource payloads are written to embedded storage.

When enabled, Spectre masks sensitive values in:

- `Secret.data` and `Secret.stringData`
- `ConfigMap.data` and `ConfigMap.binaryData`
- explicit container `env[].value` fields in supported workload specs
- `kubectl.kubernetes.io/last-applied-configuration`

The scrubber preserves a small readable prefix or suffix so values remain partially usable for debugging without storing full plaintext values.

**Example command:**

```bash
spectre server --data-dir=./data --scrub-sensitive-data=true
```

**Behavior notes:**

- Applies to live watcher ingestion before events are persisted
- Applies to startup imports before imported events are processed by the embedded backend
- Does not rewrite historical data already stored on disk
- Leaves references such as `valueFrom` unchanged because they do not contain literal secret values

## Startup Import

**Flag:** `--import-path`
**Type:** String (file or directory path)
**Default:** `""` (disabled)

**Purpose:** Import historical events at startup before the embedded backend begins serving requests.

Spectre accepts these startup import formats:

- Native Spectre JSON: `{"events":[...]}`
- Kubernetes audit `Event`
- Kubernetes audit `EventList`
- Line-delimited Kubernetes audit JSON in `.jsonl` or `.log` files

When importing official Kubernetes audit logs, Spectre:

- keeps mutating requests only: `create`, `update`, `patch`, `apply`, `delete`, `deletecollection`
- skips read-only requests such as `get`, `list`, `watch`, `proxy`, and `connect`
- uses `responseObject` first, then `requestObject`, as the best-effort resource snapshot
- warns and skips mutating audit entries that do not contain enough object payload or identity data to build a Spectre event

**Examples:**

```bash
spectre server --import-path=/backups/events-2025-12-11.json
spectre server --import-path=/backups/kube-apiserver-audit/
```
