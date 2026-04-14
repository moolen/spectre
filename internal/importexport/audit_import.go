package importexport

import (
	"crypto/sha1" //nolint:gosec // deterministic non-crypto hash for stable synthetic IDs
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/models"
)

type auditEvent struct {
	Kind                     string          `json:"kind"`
	APIVersion               string          `json:"apiVersion"`
	AuditID                  string          `json:"auditID"`
	Stage                    string          `json:"stage"`
	Verb                     string          `json:"verb"`
	RequestURI               string          `json:"requestURI"`
	StageTimestamp           string          `json:"stageTimestamp"`
	RequestReceivedTimestamp string          `json:"requestReceivedTimestamp"`
	ObjectRef                auditObjectRef  `json:"objectRef"`
	ResponseObject           json.RawMessage `json:"responseObject"`
	RequestObject            json.RawMessage `json:"requestObject"`
}

type auditObjectRef struct {
	Resource   string `json:"resource"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
	APIGroup   string `json:"apiGroup"`
	APIVersion string `json:"apiVersion"`
}

type auditEventList struct {
	Kind       string       `json:"kind"`
	APIVersion string       `json:"apiVersion"`
	Items      []auditEvent `json:"items"`
}

func parseAuditObjectPayload(data []byte) (*parseResult, error) {
	var meta struct {
		Kind       string `json:"kind"`
		APIVersion string `json:"apiVersion"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	if !isAuditAPIVersion(meta.APIVersion) {
		return nil, fmt.Errorf("unsupported audit apiVersion %q", meta.APIVersion)
	}

	switch meta.Kind {
	case "Event":
		var evt auditEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to parse audit Event: %w", err)
		}
		return normalizeAuditEvents([]auditEvent{evt}), nil
	case "EventList":
		var list auditEventList
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("failed to parse audit EventList: %w", err)
		}
		return normalizeAuditEvents(list.Items), nil
	default:
		return nil, fmt.Errorf("unsupported audit kind %q", meta.Kind)
	}
}

func parseAuditJSONLPayload(data []byte) (*parseResult, error) {
	lines := strings.Split(string(data), "\n")
	events := make([]auditEvent, 0, len(lines))

	for i, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		var evt auditEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			return nil, fmt.Errorf("failed to parse audit JSONL line %d: %w", i+1, err)
		}
		if evt.Kind != "Event" {
			return nil, fmt.Errorf("unsupported audit JSONL line %d kind %q", i+1, evt.Kind)
		}
		if !isAuditAPIVersion(evt.APIVersion) {
			return nil, fmt.Errorf("unsupported audit JSONL line %d apiVersion %q", i+1, evt.APIVersion)
		}

		events = append(events, evt)
	}

	return normalizeAuditEvents(events), nil
}

func normalizeAuditEvents(auditEvents []auditEvent) *parseResult {
	result := &parseResult{
		Events:   make([]*models.Event, 0, len(auditEvents)),
		Warnings: []string{},
	}

	for _, auditEvt := range auditEvents {
		if !shouldKeepAuditStage(auditEvt.Stage) {
			continue
		}

		eventType, keep := mapAuditVerbToEventType(auditEvt.Verb)
		if !keep {
			continue
		}

		data := preferredAuditObjectPayload(auditEvt)
		if len(data) == 0 && eventType != models.EventTypeDelete {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"skipped audit event for %q: missing responseObject/requestObject payload",
				auditObjectIdentifier(auditEvt),
			))
			continue
		}

		timestamp, ok := parseAuditTimestamp(auditEvt)
		if !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"skipped audit event for %q: missing or invalid stageTimestamp/requestReceivedTimestamp",
				auditObjectIdentifier(auditEvt),
			))
			continue
		}

		resource, ok := buildAuditResourceMetadata(auditEvt, data)
		if !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"skipped audit event for %q: insufficient object identity in objectRef",
				auditObjectIdentifier(auditEvt),
			))
			continue
		}

		evt := &models.Event{
			ID:        uuid.NewString(),
			Timestamp: timestamp,
			Type:      eventType,
			Resource:  resource,
			Data:      data,
		}

		result.Events = append(result.Events, evt)
	}

	result.Warnings = dedupeWarnings(result.Warnings)
	return result
}

func shouldKeepAuditStage(stage string) bool {
	switch stage {
	case "", "ResponseComplete", "Panic":
		return true
	default:
		return false
	}
}

func mapAuditVerbToEventType(verb string) (models.EventType, bool) {
	switch strings.ToLower(strings.TrimSpace(verb)) {
	case "create":
		return models.EventTypeCreate, true
	case "update", "patch", "apply":
		return models.EventTypeUpdate, true
	case "delete", "deletecollection":
		return models.EventTypeDelete, true
	case "get", "list", "watch", "proxy", "connect":
		return "", false
	default:
		return "", false
	}
}

func preferredAuditObjectPayload(auditEvt auditEvent) json.RawMessage {
	if len(auditEvt.ResponseObject) > 0 && string(auditEvt.ResponseObject) != "null" {
		return auditEvt.ResponseObject
	}
	if len(auditEvt.RequestObject) > 0 && string(auditEvt.RequestObject) != "null" {
		return auditEvt.RequestObject
	}
	return nil
}

func parseAuditTimestamp(auditEvt auditEvent) (int64, bool) {
	ts := strings.TrimSpace(auditEvt.StageTimestamp)
	if ts == "" {
		ts = strings.TrimSpace(auditEvt.RequestReceivedTimestamp)
	}
	if ts == "" {
		return 0, false
	}

	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return 0, false
	}
	return parsed.UnixNano(), true
}

func buildAuditResourceMetadata(auditEvt auditEvent, payload json.RawMessage) (models.ResourceMetadata, bool) {
	payloadMeta := extractAuditPayloadMetadata(payload)
	group, version := parseAuditGroupVersion(auditEvt.ObjectRef.APIGroup, auditEvt.ObjectRef.APIVersion)
	if version == "" {
		version = payloadMeta.Version
	}
	if group == "" {
		group = payloadMeta.Group
	}

	name := strings.TrimSpace(auditEvt.ObjectRef.Name)
	if name == "" {
		name = payloadMeta.Name
	}

	namespace := strings.TrimSpace(auditEvt.ObjectRef.Namespace)
	if namespace == "" {
		namespace = payloadMeta.Namespace
	}

	kind := payloadMeta.Kind
	if kind == "" {
		kind = inferKindFromResource(auditEvt.ObjectRef.Resource)
	}

	if version == "" || kind == "" || name == "" {
		return models.ResourceMetadata{}, false
	}

	uid := strings.TrimSpace(auditEvt.ObjectRef.UID)
	if uid == "" {
		uid = payloadMeta.UID
	}
	if uid == "" {
		uid = synthesizeAuditUID(group, version, kind, namespace, name, auditEvt.ObjectRef.Resource)
	}

	return models.ResourceMetadata{
		Group:     group,
		Version:   version,
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		UID:       uid,
	}, true
}

type auditPayloadMetadata struct {
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
	UID       string
}

func extractAuditPayloadMetadata(payload json.RawMessage) auditPayloadMetadata {
	if len(payload) == 0 {
		return auditPayloadMetadata{}
	}

	var m struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			UID       string `json:"uid"`
		} `json:"metadata"`
	}

	if err := json.Unmarshal(payload, &m); err != nil {
		return auditPayloadMetadata{}
	}

	group, version := splitAPIVersion(strings.TrimSpace(m.APIVersion))
	return auditPayloadMetadata{
		Group:     group,
		Version:   version,
		Kind:      strings.TrimSpace(m.Kind),
		Name:      strings.TrimSpace(m.Metadata.Name),
		Namespace: strings.TrimSpace(m.Metadata.Namespace),
		UID:       strings.TrimSpace(m.Metadata.UID),
	}
}

func isAuditAPIVersion(apiVersion string) bool {
	return strings.HasPrefix(strings.TrimSpace(apiVersion), "audit.k8s.io/")
}

func parseAuditGroupVersion(apiGroup, apiVersion string) (string, string) {
	apiGroup = strings.TrimSpace(apiGroup)
	apiVersion = strings.TrimSpace(apiVersion)
	if apiVersion == "" {
		return apiGroup, ""
	}

	group, version := splitAPIVersion(apiVersion)
	if group == "" {
		group = apiGroup
	}
	return group, version
}

func splitAPIVersion(apiVersion string) (string, string) {
	if apiVersion == "" {
		return "", ""
	}
	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}

func inferKindFromResource(resource string) string {
	resource = strings.TrimSpace(strings.ToLower(resource))
	if resource == "" {
		return ""
	}

	known := map[string]string{
		"configmaps":                      "ConfigMap",
		"daemonsets":                      "DaemonSet",
		"deployments":                     "Deployment",
		"endpointslices":                  "EndpointSlice",
		"horizontalpodautoscalers":        "HorizontalPodAutoscaler",
		"ingresses":                       "Ingress",
		"networkpolicies":                 "NetworkPolicy",
		"persistentvolumeclaims":          "PersistentVolumeClaim",
		"persistentvolumes":               "PersistentVolume",
		"poddisruptionbudgets":            "PodDisruptionBudget",
		"priorityclasses":                 "PriorityClass",
		"replicasets":                     "ReplicaSet",
		"replicationcontrollers":          "ReplicationController",
		"resourcequotas":                  "ResourceQuota",
		"runtimeclasses":                  "RuntimeClass",
		"serviceaccounts":                 "ServiceAccount",
		"storageclasses":                  "StorageClass",
		"validatingwebhookconfigurations": "ValidatingWebhookConfiguration",
	}
	if kind, ok := known[resource]; ok {
		return kind
	}

	singular := resource
	switch {
	case strings.HasSuffix(resource, "ies") && len(resource) > 3:
		singular = resource[:len(resource)-3] + "y"
	case strings.HasSuffix(resource, "ses") && len(resource) > 3:
		singular = resource[:len(resource)-2]
	case strings.HasSuffix(resource, "s") && len(resource) > 1:
		singular = resource[:len(resource)-1]
	}

	if singular == "" {
		return ""
	}
	return strings.ToUpper(singular[:1]) + singular[1:]
}

func synthesizeAuditUID(group, version, kind, namespace, name, resource string) string {
	identity := strings.Join([]string{
		group,
		version,
		kind,
		namespace,
		name,
		resource,
	}, "|")
	sum := sha1.Sum([]byte(identity))
	return "audit-" + hex.EncodeToString(sum[:8])
}

func auditObjectIdentifier(auditEvt auditEvent) string {
	if auditEvt.ObjectRef.Name != "" {
		if auditEvt.ObjectRef.Namespace != "" {
			return auditEvt.ObjectRef.Namespace + "/" + auditEvt.ObjectRef.Name
		}
		return auditEvt.ObjectRef.Name
	}
	if auditEvt.RequestURI != "" {
		return auditEvt.RequestURI
	}
	if auditEvt.AuditID != "" {
		return auditEvt.AuditID
	}
	return "unknown"
}

func dedupeWarnings(warnings []string) []string {
	if len(warnings) < 2 {
		return warnings
	}

	seen := make(map[string]struct{}, len(warnings))
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if _, ok := seen[warning]; ok {
			continue
		}
		seen[warning] = struct{}{}
		result = append(result, warning)
	}
	return result
}
