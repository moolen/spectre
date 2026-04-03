package falkor

import (
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
)

func appendRelatedResource(
	related map[string][]analysisstore.RelatedResourceData,
	resourceUID string,
	resource graph.ResourceIdentity,
	relationshipType string,
) {
	for _, existing := range related[resourceUID] {
		if existing.Resource.UID == resource.UID && existing.RelationshipType == relationshipType {
			return
		}
	}

	related[resourceUID] = append(related[resourceUID], analysisstore.RelatedResourceData{
		Resource:         resource,
		RelationshipType: relationshipType,
		Events:           []analysisstore.ChangeEventInfo{},
	})
}

func appendIngressReference(
	related map[string][]analysisstore.RelatedResourceData,
	resourceUID string,
	ingress graph.ResourceIdentity,
	referenceTargetUID string,
) {
	for _, existing := range related[resourceUID] {
		if existing.Resource.UID == ingress.UID && existing.RelationshipType == edgeTypeIngressRef {
			return
		}
	}

	related[resourceUID] = append(related[resourceUID], analysisstore.RelatedResourceData{
		Resource:           ingress,
		RelationshipType:   edgeTypeIngressRef,
		Events:             []analysisstore.ChangeEventInfo{},
		ReferenceTargetUID: referenceTargetUID,
	})
}
