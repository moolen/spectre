---
phase: 14-ui-helm-chart
plan: 01
subsystem: ui
tags: [react, typescript, logzio, helm, kubernetes, secrets, integration-form]

# Dependency graph
requires:
  - phase: 13-01
    provides: Logzio integration complete with 3 MCP tools (overview, logs, patterns)
  - phase: 02-03
    provides: IntegrationConfigForm pattern with conditional rendering by type
  - phase: 11-04
    provides: Helm extraVolumes pattern and RBAC setup
provides:
  - Logzio configuration form with region selector and SecretRef fields
  - Kubernetes Secret mounting documentation with rotation workflow
  - Complete v1.2 milestone: Logzio integration fully configurable via UI
affects: [future-integrations-ui-forms, kubernetes-deployment, secret-management-docs]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SecretRef form pattern: separate Secret Name and Key fields in Authentication section"
    - "Region selector pattern: native select element with code + name display"
    - "Helm documentation pattern: in-line commented examples for copy-paste deployment"
    - "Secret rotation workflow: create v2 → update extraVolumes.secretName → helm upgrade"

key-files:
  created: []
  modified:
    - ui/src/components/IntegrationConfigForm.tsx
    - chart/values.yaml

key-decisions:
  - "Region selector as dropdown (not freeform URL) with 5 regions (US, EU, UK, AU, CA)"
  - "SecretRef split into separate Secret Name and Key fields for clarity"
  - "Authentication section visually grouped with border and background"
  - "Helm Secret mounting as commented example (not new helper template)"
  - "Copy-paste workflow documentation: kubectl command → YAML → UI config → rotation"

patterns-established:
  - "SecretRef UI pattern: Authentication section with secretName and key fields"
  - "Regional endpoint pattern: Select with human-readable labels (US, EU, UK, AU, CA)"
  - "Helm Secret documentation: 4-step workflow (create → mount → configure → rotate)"
  - "Security best practices: defaultMode: 0400, readOnly: true in volume mounts"

# Metrics
duration: 2min
completed: 2026-01-22
---

# Phase 14 Plan 01: UI and Helm Chart Summary

**Logzio configuration form with region dropdown and SecretRef fields, plus Kubernetes Secret mounting documentation for production deployment**

## Performance

- **Duration:** ~2 minutes (human checkpoint verification time)
- **Started:** 2026-01-22T17:59:00Z
- **Completed:** 2026-01-22T18:01:00Z
- **Tasks:** 2 (Task 1, Task 3) + 1 checkpoint
- **Files modified:** 2

## Accomplishments

- Logzio configuration form in UI with region selector (5 regions) and SecretRef fields
- Authentication section with bordered visual grouping (Secret Name, Key)
- Helm chart values.yaml includes copy-paste Secret mounting example
- Complete 4-step workflow documented: create Secret → mount → configure → rotate
- v1.2 milestone complete: Logzio integration fully configurable via UI with Kubernetes secret management

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Logzio form section with region dropdown and SecretRef fields** - `913a5a9` (feat)
   - Add "Logz.io" option to Type dropdown
   - Region selector with 5 regions (US, EU, UK, AU, CA) and placeholder text
   - Authentication section with bordered background (visual grouping)
   - Secret Name field (placeholder: logzio-creds)
   - Key field (placeholder: api-token)
   - Event handlers: handleRegionChange, handleSecretNameChange, handleSecretKeyChange
   - Nested config structure matches backend types (apiTokenRef.secretName, apiTokenRef.key)
   - Follows existing VictoriaLogs pattern for consistency (inline styles, help text)

2. **Task 3: Add Helm Secret mounting documentation** - `0722004` (docs)
   - Commented Secret mounting example in values.yaml after extraVolumeMounts
   - Step 1: kubectl create secret command with proper syntax
   - Step 2: extraVolumes and extraVolumeMounts YAML (commented, ready to uncomment)
   - Step 3: UI configuration instructions (region + SecretRef fields)
   - Step 4: Secret rotation workflow (create v2 → update → helm upgrade → auto-reload)
   - Security best practices: defaultMode: 0400, readOnly: true
   - Copy-paste friendly for platform engineers

3. **Checkpoint: Human verification of UI form and documentation** - APPROVED
   - User verified Logzio form renders correctly with all fields
   - User confirmed region dropdown has 5 options
   - User confirmed Authentication section layout and field interactions
   - User confirmed Helm documentation is copy-paste ready

## Files Created/Modified

### Modified

- **ui/src/components/IntegrationConfigForm.tsx** (+210 lines)
  - Add "Logz.io" option to Type dropdown (line 138)
  - Region selector with 5 options (us, eu, uk, au, ca)
  - Authentication section with bordered background
  - Secret Name and Key input fields with help text
  - handleRegionChange updates config.config.region
  - handleSecretNameChange updates config.config.apiTokenRef.secretName
  - handleSecretKeyChange updates config.config.apiTokenRef.key
  - Layout matches existing VictoriaLogs pattern

- **chart/values.yaml** (+30 lines)
  - Commented Secret mounting example after extraVolumeMounts (line 329)
  - kubectl create secret command with --from-literal
  - extraVolumes with secret.secretName and defaultMode: 0400
  - extraVolumeMounts with mountPath and readOnly: true
  - 4-step workflow: create → mount → configure → rotate
  - Secret rotation pattern: create v2 → update secretName → helm upgrade

## Decisions Made

**1. Region selector as dropdown (not freeform URL)**
- **Rationale:** Logz.io has 5 fixed regional endpoints, dropdown prevents typos and makes selection clear
- **Impact:** User picks from "US (United States)", "EU (Europe)", "UK (United Kingdom)", "AU (Australia)", "CA (Canada)"

**2. SecretRef split into separate Secret Name and Key fields**
- **Rationale:** Kubernetes Secrets have name and key structure, separate fields make this explicit and reduce confusion
- **Impact:** Two text fields instead of one compound field, clearer for platform engineers

**3. Authentication section visually grouped**
- **Rationale:** Secret configuration is distinct from connection settings (region), visual separation improves form scannability
- **Impact:** Bordered background section containing Secret Name and Key fields

**4. Helm Secret mounting as commented example (not helper template)**
- **Rationale:** Target audience (platform engineers) familiar with extraVolumes pattern, commented examples are copy-paste friendly
- **Impact:** Users uncomment and fill in values, no new Helm abstractions introduced

**5. Copy-paste workflow documentation**
- **Rationale:** Platform engineers want actionable examples, not verbose explanations
- **Impact:** kubectl command → YAML → UI config → rotation workflow in ~30 lines

## Deviations from Plan

None - plan executed exactly as written.

All implementation matched plan specifications:
- Logzio option added to Type dropdown
- Region selector with 5 regions (US, EU, UK, AU, CA)
- Authentication section with Secret Name and Key fields
- Event handlers update nested config object structure
- Helm values.yaml has commented Secret mounting example after extraVolumeMounts
- Documentation includes kubectl command, YAML, UI config, and rotation workflow
- Security best practices included (defaultMode: 0400, readOnly: true)
- Human verification checkpoint completed with user approval

## Issues Encountered

None - implementation proceeded smoothly. UI form rendered correctly on first attempt, all field interactions worked as expected. Helm documentation syntax validated successfully.

## User Setup Required

None - configuration now done via UI.

**For production deployment:**

1. Create Kubernetes Secret in Spectre's namespace:
   ```bash
   kubectl create secret generic logzio-creds \
     --from-literal=api-token=YOUR_TOKEN_HERE \
     --namespace monitoring
   ```

2. Uncomment and configure extraVolumes/extraVolumeMounts in values.yaml (see chart/values.yaml lines 329-365)

3. Deploy with Helm:
   ```bash
   helm upgrade spectre ./chart --install
   ```

4. Configure Logzio integration in UI:
   - Type: Logz.io
   - Region: Select your Logz.io account region
   - Secret Name: logzio-creds
   - Key: api-token

5. Test connection before saving

See chart/values.yaml for complete Secret rotation workflow.

## Verification Results

**UI Form Verification (Human Checkpoint):**
- Logzio appears in Type dropdown ✓
- Region dropdown renders with 5 options and placeholder ✓
- Authentication section renders with bordered background ✓
- Secret Name field renders with placeholder "logzio-creds" ✓
- Key field renders with placeholder "api-token" ✓
- Help text displays under each field ✓
- Field interactions update state correctly ✓
- Layout matches VictoriaLogs pattern (consistent spacing, styling) ✓

**Helm Chart Documentation Verification:**
- Commented example appears after extraVolumeMounts ✓
- kubectl command syntax correct ✓
- YAML indentation valid ✓
- 4-step workflow documented (create → mount → configure → rotate) ✓
- Security best practices included (defaultMode: 0400, readOnly: true) ✓
- Copy-paste friendly format ✓

**Connection Test (Existing Infrastructure):**
- IntegrationModal.tsx POST /api/config/integrations/test endpoint ✓
- Backend validates SecretRef existence and API token ✓
- Specific error messages: "Secret 'x' not found", "401 Unauthorized" ✓
- No additional work required (infrastructure from Phase 11) ✓

## v1.2 Milestone Complete

**All 5 requirements satisfied:**

1. **CONF-02:** UI displays Logzio configuration form with region selector ✓
   - Region dropdown with 5 options (US, EU, UK, AU, CA)
   - SecretRef fields (Secret Name, Key) in Authentication section

2. **CONF-03:** Connection test validates token before saving ✓
   - Existing /api/config/integrations/test endpoint handles validation
   - Specific error messages for authentication failures and missing Secrets

3. **HELM-01:** Helm values include extraVolumes example ✓
   - Commented example in values.yaml after extraVolumeMounts
   - Follows existing Helm patterns

4. **HELM-02:** Documentation covers secret rotation workflow ✓
   - 4-step workflow: create v2 → update secretName → helm upgrade → auto-reload
   - SecretWatcher from Phase 11 handles hot-reload automatically

5. **HELM-03:** Example Kubernetes Secret manifest ✓
   - kubectl create secret command with correct syntax
   - Ready for copy-paste deployment

**v1.2 Logz.io Integration Deliverables:**
- HTTP client with multi-region support (Phase 10)
- Kubernetes-native secret hot-reload (Phase 11)
- MCP tools: overview, logs, patterns (Phases 12-13)
- UI configuration form (Phase 14)
- Helm chart with secret mounting (Phase 14)

**Platform engineers can now:**
- Configure Logzio integrations entirely via UI (no manual API calls)
- Deploy with Kubernetes Secrets following documented workflow
- Rotate credentials without pod restarts (SecretWatcher hot-reload)
- AI assistants can explore Logzio logs with progressive disclosure (overview → logs → patterns)

## Next Phase Readiness

**v1.2 milestone shipped:**
- All planned phases complete (Phases 10-14)
- All 21 requirements satisfied
- Logzio integration production-ready

**No further phases planned for v1.2.**

**Potential future work (out of scope for v1.2):**
- Additional log backend integrations (follow Logzio pattern)
- Secret listing/picker UI (requires additional RBAC)
- Multi-account support in single integration
- Integration-specific MCP tools (e.g., Datadog metrics, Sentry issues)

**No blockers.**

---
*Phase: 14-ui-helm-chart*
*Completed: 2026-01-22*
