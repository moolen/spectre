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
