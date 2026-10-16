package embeddedstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/stretchr/testify/require"
)

// referenceDetectChangeKinds is the diff-based classification that change fingerprints
// replaced; it re-parses and diffs both payloads.
func referenceDetectChangeKinds(previousData, currentData []byte) (bool, bool) {
	if len(previousData) == 0 || len(currentData) == 0 {
		return false, false
	}
	previousObject := parseObject(previousData)
	currentObject := parseObject(currentData)
	kind := getString(currentObject, "kind")

	fieldChanged := func(key string) bool {
		return !reflect.DeepEqual(previousObject[key], currentObject[key])
	}
	configChanged := false
	switch kind {
	case "ConfigMap", "Secret":
		configChanged = fieldChanged("data") || fieldChanged("binaryData") || fieldChanged("stringData")
	case "Role", "ClusterRole":
		configChanged = fieldChanged("rules") || fieldChanged("aggregationRule")
	case "RoleBinding", "ClusterRoleBinding":
		configChanged = fieldChanged("roleRef") || fieldChanged("subjects")
	}
	statusChanged := fieldChanged("status")

	diffs, err := analysis.ComputeJSONDiff(previousData, currentData)
	if err != nil {
		return configChanged, statusChanged
	}
	for _, diff := range diffs {
		if bytes.HasPrefix([]byte(diff.Path), []byte("/spec")) || bytes.HasPrefix([]byte(diff.Path), []byte("spec")) {
			configChanged = true
		}
		if bytes.HasPrefix([]byte(diff.Path), []byte("/status")) || bytes.HasPrefix([]byte(diff.Path), []byte("status")) {
			statusChanged = true
		}
	}
	return configChanged, statusChanged
}

func fingerprintChangeKinds(previousData, currentData []byte) (bool, bool) {
	previous := newRecordChangeState(previousData, parseObject(previousData))
	current := newRecordChangeState(currentData, parseObject(currentData))
	return changeKindsFromStates(previous, current)
}

func randomJSONValue(rng *rand.Rand, depth int) any {
	switch choice := rng.Intn(7); {
	case depth > 2 || choice == 0:
		return fmt.Sprintf("v%d", rng.Intn(3))
	case choice == 1:
		return float64(rng.Intn(3))
	case choice == 2:
		return rng.Intn(2) == 0
	case choice == 3:
		return nil
	case choice == 4:
		items := make([]any, rng.Intn(3))
		for i := range items {
			items[i] = randomJSONValue(rng, depth+1)
		}
		return items
	default:
		object := map[string]any{}
		for i := rng.Intn(3); i > 0; i-- {
			object[fmt.Sprintf("k%d", rng.Intn(3))] = randomJSONValue(rng, depth+1)
		}
		return object
	}
}

func mutateTopLevel(rng *rand.Rand, object map[string]any, keys []string) map[string]any {
	mutated := make(map[string]any, len(object))
	for key, value := range object {
		mutated[key] = value
	}
	for i := rng.Intn(3); i >= 0; i-- {
		key := keys[rng.Intn(len(keys))]
		switch rng.Intn(3) {
		case 0:
			delete(mutated, key)
		default:
			mutated[key] = randomJSONValue(rng, 0)
		}
	}
	return mutated
}

func TestChangeFingerprints_MatchDiffBasedClassification(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	kinds := []string{"Pod", "ConfigMap", "Secret", "Role", "ClusterRoleBinding", ""}
	keys := []string{"spec", "status", "specExtra", "statusDetail", "data", "binaryData", "stringData", "rules", "aggregationRule", "roleRef", "subjects", "metadata", "other"}

	for i := 0; i < 5000; i++ {
		kind := kinds[rng.Intn(len(kinds))]
		previous := map[string]any{}
		if kind != "" {
			previous["kind"] = kind
		}
		previous = mutateTopLevel(rng, previous, keys)
		current := mutateTopLevel(rng, previous, keys)
		if rng.Intn(4) == 0 {
			current = previous
		}
		if kind != "" {
			current["kind"] = kind
		}
		previousData, err := json.Marshal(previous)
		require.NoError(t, err)
		currentData, err := json.Marshal(current)
		require.NoError(t, err)

		wantConfig, wantStatus := referenceDetectChangeKinds(previousData, currentData)
		gotConfig, gotStatus := fingerprintChangeKinds(previousData, currentData)
		require.Equal(t, wantConfig, gotConfig, "config changed\nprevious: %s\ncurrent:  %s", previousData, currentData)
		require.Equal(t, wantStatus, gotStatus, "status changed\nprevious: %s\ncurrent:  %s", previousData, currentData)
	}
}

func TestChangeFingerprints_MissingOrInvalidPayloadsNeverCountAsChanges(t *testing.T) {
	valid := []byte(`{"kind":"Pod","spec":{"a":1},"status":{"phase":"Running"}}`)
	for _, pair := range [][2][]byte{{nil, valid}, {valid, nil}, {[]byte(`{not json`), valid}, {valid, []byte(`[1,2]`)}} {
		wantConfig, wantStatus := referenceDetectChangeKinds(pair[0], pair[1])
		gotConfig, gotStatus := fingerprintChangeKinds(pair[0], pair[1])
		require.Equal(t, []bool{wantConfig, wantStatus}, []bool{gotConfig, gotStatus}, "%s -> %s", pair[0], pair[1])
	}
}
