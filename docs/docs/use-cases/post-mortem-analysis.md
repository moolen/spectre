---
title: Post-Mortem Analysis
description: Generate comprehensive incident reports using Spectre's historical event data
keywords: [post-mortem, incident, analysis, report, rca, root cause, documentation]
---

# Post-Mortem Analysis

Create thorough post-mortem reports using Spectre's complete event history and timeline reconstruction capabilities.

## Overview

After an incident is resolved, post-mortem analysis helps:
- **Document what happened** - Complete timeline with exact timestamps
- **Identify root causes** - Event correlation reveals why it happened
- **Assess impact** - Understand scope and duration of the incident
- **Prevent recurrence** - Actionable recommendations based on patterns
- **Share learnings** - Exportable reports for team knowledge

**Time saved**: 30-60 minutes per post-mortem through automated timeline generation and structured analysis.

## Post-Mortem Structure

A complete post-mortem includes:

1. **Incident Summary** - Brief overview and timeline
2. **Timeline** - Chronological events with timestamps
3. **Root Cause Analysis** - Primary cause and contributing factors
4. **Impact Assessment** - Affected services, downtime, user impact
5. **Resolution Steps** - What was done to resolve
6. **Recommendations** - Preventive measures and improvements
7. **Action Items** - Specific tasks with owners and deadlines

Spectre provides data for all sections except resolution steps (documented during incident) and action items (defined after analysis).

## Step-by-Step Guide

### Step 1: Define Incident Window

**Identify the time range** for analysis:

- **Start time**: When symptoms first appeared (or 15-30 minutes before for precursors)
- **End time**: When service was fully restored and stable
- **Namespace**: Affected namespace(s)

**Example**:
```
Incident: API service outage
Start: 2024-12-12 14:00:00 UTC (symptoms appeared)
End: 2024-12-12 14:30:00 UTC (service restored)
Namespace: production
```

**Convert to Unix timestamps** (for API queries):
```bash
START=$(date -u -d "2024-12-12 14:00:00" +%s)  # 1702389600
END=$(date -u -d "2024-12-12 14:30:00" +%s)    # 1702391400
```

### Step 2: Gather Event Data

#### Query All Events in Window

**Spectre UI**:
1. Navigate to Spectre UI
2. Set time range: Start → End
3. Filter by namespace: `namespace:production`
4. Export timeline view

**Spectre API**:
```bash
curl "http://spectre:8080/api/search?query=namespace:production&start=$START&end=$END" | jq . > incident-events.json
```

#### Query High-Impact Resources

**Identify resources with status changes**:
```
Query: status:Error OR status:Warning
Namespace: production
Time range: Incident window
```

**Common resource types to check**:
- Deployments (rollout issues)
- Pods (failures, restarts)
- ConfigMaps/Secrets (config changes)
- Services/Endpoints (connectivity issues)
- ReplicaSets (scaling problems)

### Step 3: Build Timeline

#### Extract Key Events

**Look for significant events**:
1. **Precursors** (15-30 min before symptoms)
   - Deployments updates
   - Config changes
   - Scaling events

2. **Symptom onset** (when issues appeared)
   - First pod failures
   - Error events spike
   - Status transitions to Error

3. **Investigation** (during incident)
   - Multiple resources transitioning to error
   - Cascading failures
   - Attempted fixes

4. **Resolution** (recovery phase)
   - Successful rollback/fix
   - Pods returning to Running
   - Services becoming healthy

#### Format Timeline

**Example timeline format**:
```markdown
## Timeline

[14:00:05] **ConfigMap/production/api-config** - Deleted
           Event: DELETE operation by user@example.com

[14:02:18] **Deployment/production/api-server** - Triggered rolling update
           (ConfigMap referenced in pod spec)

[14:02:45] **Pod/production/api-server-7d9f8c5b-x7k2p** - Status: Running → Error
           Event: "ConfigMap api-config not found"

[14:03:00] **Service/production/api-server** - No ready endpoints
           All pods unhealthy

[14:15:30] **ConfigMap/production/api-config** - Created (restored from backup)
           Event: CREATE operation by ops-team

[14:15:55] **Pod/production/api-server-9c8f7b6d-a3m5n** - Status: Running
           New pod started successfully

[14:16:10] **Service/production/api-server** - Endpoints restored
           Service traffic resumed
```

### Step 4: Root Cause Analysis

#### Identify Primary Cause

**Correlation analysis**:
1. What changed immediately before the incident?
2. What failed first?
3. What error messages appeared?

**Example RCA**:
```markdown
## Root Cause Analysis

**Primary Cause**: ConfigMap "api-config" was accidentally deleted

**Timeline correlation**:
- [14:00:05] ConfigMap deleted
- [14:02:45] Pods started failing (2 minutes 40 seconds later)
- Error message: "ConfigMap api-config not found"

**Why it happened**:
- Manual kubectl delete command executed in wrong namespace
- No RBAC restrictions preventing ConfigMap deletion
- No validation webhook or policy guard
```

#### Identify Contributing Factors

**Look for systemic issues**:
```markdown
**Contributing Factors**:
1. No version control for ConfigMaps (no GitOps)
2. Deployment requires ConfigMap but has no failure handling
3. No alerting on ConfigMap deletions
4. Manual restoration took 15 minutes (no automated backup)
5. No documentation for ConfigMap recovery procedure
```

### Step 5: Impact Assessment

#### Service Impact

**Metrics to include**:
- **Downtime duration**: 15 minutes 55 seconds (from first failure to full recovery)
- **Error rate**: 100% during outage
- **Affected services**: api-server (complete unavailability)
- **User impact**: API unavailable, ~500 affected users (estimated from traffic)

**Query Spectre for affected resources**:
```
Query: status:Error OR status:Warning
Time range: Incident window
Count: Unique resources
```

#### Resource Impact

**List all affected resources**:
```markdown
**Resources Affected**:
- 1 ConfigMap (deleted, then recreated)
- 1 Deployment (failed rollout)
- 3 Pods (failed to start, terminated)
- 1 Service (no endpoints available)
- 0 customer data lost
```

### Step 6: Generate Report

#### Use MCP for Automated Reports

**With Claude Desktop + MCP**:
```
You: Run post-mortem analysis for the incident yesterday from 14:00 to 14:30 UTC in production namespace

Claude: [Executes post_mortem_incident_analysis prompt]

## Incident Post-Mortem Report

[Automatically generates complete report with:]
- Executive summary
- Complete timeline
- Root cause analysis with evidence
- Impact assessment
- Recommendations
- Data gaps to investigate further
```

**MCP benefits**:
- Automated timeline generation
- Correlation analysis
- Structured report format
- Evidence-based conclusions

**Learn more**: [MCP Post-Mortem Prompt](../mcp-integration/prompts-reference/post-mortem.md)

#### Manual Report Generation

**Template structure**:
```markdown
# Incident Post-Mortem: [Title]

**Date**: YYYY-MM-DD
**Duration**: XX minutes
**Severity**: Critical/High/Medium/Low
**Status**: Resolved

## Summary

[1-2 paragraph overview]

## Timeline

[Chronological events from Spectre]

## Root Cause

[Primary cause + contributing factors]

## Impact

[Downtime, affected users, business impact]

## Resolution

[Steps taken to resolve]

## Lessons Learned

**What went well**:
- Quick detection (alert fired within X minutes)
- Backup available for ConfigMap

**What went wrong**:
- No prevention mechanisms
- Manual recovery took too long

## Recommendations

**Immediate** (this week):
1. [Action item with owner]
2. [Action item with owner]

**Short-term** (this month):
1. [Action item with owner]

**Long-term** (this quarter):
1. [Action item with owner]

## Action Items

- [ ] @owner: Task description (by YYYY-MM-DD)
- [ ] @owner: Task description (by YYYY-MM-DD)

## Appendix

[Spectre query links, exported event data]
```

## Best Practices

### ✅ Do

- **Include precursor events**: Look 15-30 minutes before symptom onset
- **Use exact timestamps**: Spectre provides microsecond precision
- **Document evidence**: Link to Spectre queries or export event data
- **Focus on facts**: Only report events observed by Spectre
- **Identify patterns**: Look for recurring issues or similar past incidents
- **Make actionable recommendations**: Specific, assignable, with deadlines
- **Share widely**: Post-mortems are learning opportunities

### ❌ Don't

- **Don't blame individuals**: Focus on systemic improvements
- **Don't skip small incidents**: Even 5-minute outages deserve analysis
- **Don't rely on memory**: Use Spectre's factual event data
- **Don't ignore contributing factors**: Address systemic issues, not just immediate cause
- **Don't forget follow-up**: Track action items to completion
- **Don't make assumptions**: If Spectre doesn't show it, note it as hypothesis requiring verification

## Example Post-Mortem

### Scenario: ConfigMap Deletion Outage

```markdown
# Post-Mortem: API Service Outage (ConfigMap Deletion)

**Date**: 2024-12-12
**Duration**: 15 minutes 55 seconds (14:00 - 14:16 UTC)
**Severity**: Critical (100% unavailability)
**Author**: @ops-team
**Reviewers**: @dev-team, @management

## Executive Summary

Production API service experienced complete outage for ~16 minutes due to
accidental deletion of ConfigMap "api-config". Service was restored after
ConfigMap was recreated from backup. No data loss occurred.

## Timeline

**Precursor**:
[14:00:05] ConfigMap/production/api-config deleted
          (Manual kubectl command, user: ops@example.com)

**Failure Cascade**:
[14:02:18] Deployment/api-server triggered rolling update
[14:02:45] Pods began failing (ConfigMap not found)
[14:03:00] Service endpoints removed (no ready pods)
[14:03:15] First customer error reports received

**Investigation**:
[14:03:30] On-call alerted via PagerDuty
[14:05:00] Team identified missing ConfigMap via Spectre timeline
[14:10:00] Backup ConfigMap located

**Resolution**:
[14:15:30] ConfigMap recreated from backup
[14:15:55] Pods started successfully
[14:16:10] Service endpoints restored
[14:20:00] Monitoring confirmed full recovery

## Root Cause Analysis

**Primary Cause**: ConfigMap "api-config" was accidentally deleted

**Root Cause**: Operator intended to delete ConfigMap in staging namespace
but executed command in production namespace due to kubectl context not
being switched.

**Evidence** (from Spectre):
- DELETE event for ConfigMap at 14:00:05
- Pod failures started exactly 2min 40sec later
- Error message: "Error: configmap 'api-config' not found"
- Timeline shows no other changes before incident

**Contributing Factors**:
1. kubectl contexts not clearly indicated in terminal prompt
2. No RBAC restrictions on ConfigMap deletion in production
3. No GitOps (ConfigMaps not in version control)
4. No validation or confirmation required for destructive operations
5. Manual ConfigMap backup process (no automation)
6. Deployment has hard dependency on ConfigMap (no graceful degradation)

## Impact Assessment

**Service Impact**:
- Complete API unavailability: 15 minutes 55 seconds
- Customer impact: ~500 users (based on typical traffic)
- Error rate: 100% during outage
- Revenue impact: $X (estimated based on transaction rate)

**Resources Affected** (via Spectre query):
- 1 ConfigMap (deleted, recreated)
- 1 Deployment (failed rollout, recovered)
- 3 Pods (terminated, recreated)
- 1 Service (no endpoints, restored)

**Downstream Impact**:
- Mobile app showed "Service Unavailable" errors
- Partner API integrations failed
- Internal dashboards went offline

## What Went Well

- Alert fired within 3 minutes of issue
- Team quickly identified root cause using Spectre
- ConfigMap backup was available
- Service restored within 16 minutes
- No data loss occurred

## What Went Wrong

- Accidental deletion possible due to lack of safeguards
- No automated recovery mechanism
- Manual restoration took 15 minutes
- No pre-incident validation (policy enforcement)

## Recommendations

**Immediate** (this week):
1. Implement GitOps for all production ConfigMaps (FluxCD)
   - Owner: @ops-team
   - Deadline: 2024-12-18

2. Add RBAC policy restricting ConfigMap deletions in production
   - Owner: @security-team
   - Deadline: 2024-12-15

**Short-term** (this month):
3. Implement ConfigMap deletion alerts (Prometheus)
   - Owner: @monitoring-team
   - Deadline: 2024-12-22

4. Update terminal prompt to clearly show kubectl context
   - Owner: @ops-team
   - Deadline: 2024-12-20

5. Add graceful degradation to application (default config values)
   - Owner: @dev-team
   - Deadline: 2024-12-31

**Long-term** (this quarter):
6. Implement Policy Engine (Kyverno/OPA) for production namespace
   - Require confirmation for destructive operations
   - Owner: @platform-team
   - Deadline: Q1 2025

7. Automated ConfigMap backup and restore system
   - Owner: @ops-team
   - Deadline: Q1 2025

## Action Items

- [ ] @ops-team: Migrate ConfigMaps to GitOps (by 2024-12-18)
- [ ] @security-team: Implement RBAC restrictions (by 2024-12-15)
- [ ] @monitoring-team: Add ConfigMap deletion alerts (by 2024-12-22)
- [ ] @ops-team: Update terminal prompt with context (by 2024-12-20)
- [ ] @dev-team: Add config fallback logic (by 2024-12-31)
- [ ] @all: Review runbooks for ConfigMap recovery (by 2024-12-22)

## Data Sources

- Spectre timeline: http://spectre/search?start=1702389600&end=1702391400&namespace=production
- PagerDuty incident: #INC-12345
- Customer reports: Support ticket #98765
- Monitoring: Grafana dashboard during outage

## Appendix A: Detailed Event Log

[Exported from Spectre - 247 events during window]
[Attached: incident-2024-12-12-events.json]

## Appendix B: Prevention Checklist

To prevent similar incidents:
- [ ] All production ConfigMaps in Git
- [ ] RBAC prevents accidental deletion
- [ ] Alerts fire on config changes
- [ ] Applications handle missing config gracefully
- [ ] Automated backup/restore tested monthly
- [ ] Runbooks documented and accessible
```

## Automation Ideas

### Export to Jira/GitHub

```bash
# Export Spectre timeline as JSON
curl "http://spectre:8080/api/search?..." > timeline.json

# Use jq to format for Jira
jq -r '.events[] | "[\(.timestamp)] \(.kind)/\(.name): \(.message)"' timeline.json > timeline.txt

# Create Jira issue with timeline
# (Use Jira API or manually paste)
```

### Generate Metrics

```bash
# Calculate downtime from Spectre events
FIRST_ERROR=$(jq -r '.events[] | select(.status=="Error") | .timestamp' timeline.json | head -1)
LAST_SUCCESS=$(jq -r '.events[] | select(.status=="Running") | .timestamp' timeline.json | tail -1)

# Downtime = LAST_SUCCESS - FIRST_ERROR
```

### Compare with Previous Incidents

```bash
# Query similar past incidents
curl "http://spectre:8080/api/search?query=kind:ConfigMap,status:Error&start=<30_days_ago>&end=<now>"

# Identify patterns (are ConfigMap issues recurring?)
```

## Related Documentation

- [Incident Investigation](./incident-investigation.md) - Real-time troubleshooting
- [Deployment Tracking](./deployment-tracking.md) - Proactive monitoring
- [MCP Post-Mortem Prompt](../mcp-integration/prompts-reference/post-mortem.md) - Automated reports
- [Export API](../user-guide/index.md) - Export event data for reports

<!-- Source: README.md, post-mortem best practices, incident patterns -->
