package falkor

import (
	"testing"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
	"github.com/stretchr/testify/require"
)

func TestAppendRelatedResource_DeduplicatesByUIDAndRelationship(t *testing.T) {
	related := map[string][]analysisstore.RelatedResourceData{
		"pod-uid": {},
	}

	resource := graph.ResourceIdentity{
		UID:       "service-account-uid",
		Kind:      "ServiceAccount",
		Namespace: "default",
		Name:      "default",
	}

	appendRelatedResource(related, "pod-uid", resource, "USES_SERVICE_ACCOUNT")
	appendRelatedResource(related, "pod-uid", resource, "USES_SERVICE_ACCOUNT")
	appendRelatedResource(related, "pod-uid", resource, "GRANTS_TO")

	require.Len(t, related["pod-uid"], 2)
	require.Equal(t, "USES_SERVICE_ACCOUNT", related["pod-uid"][0].RelationshipType)
	require.Equal(t, "GRANTS_TO", related["pod-uid"][1].RelationshipType)
}

func TestAppendIngressReference_DeduplicatesAndPreservesTargetUID(t *testing.T) {
	related := map[string][]analysisstore.RelatedResourceData{
		"pod-uid": {},
	}

	ingress := graph.ResourceIdentity{
		UID:       "ingress-uid",
		Kind:      "Ingress",
		Namespace: "default",
		Name:      "public",
	}

	appendIngressReference(related, "pod-uid", ingress, "service-uid")
	appendIngressReference(related, "pod-uid", ingress, "service-uid")

	require.Len(t, related["pod-uid"], 1)
	require.Equal(t, edgeTypeIngressRef, related["pod-uid"][0].RelationshipType)
	require.Equal(t, "service-uid", related["pod-uid"][0].ReferenceTargetUID)
}
