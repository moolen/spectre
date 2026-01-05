package extractors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

const (
	fluxGitRepositoryAPIGroup = "source.toolkit.fluxcd.io"
	gitRepositoryKind         = "GitRepository"
)

// FluxGitRepositoryExtractor extracts relationships from Flux GitRepository resources
type FluxGitRepositoryExtractor struct {
	*BaseExtractor
}

// NewFluxGitRepositoryExtractor creates a new Flux GitRepository extractor
func NewFluxGitRepositoryExtractor() *FluxGitRepositoryExtractor {
	return &FluxGitRepositoryExtractor{
		BaseExtractor: NewBaseExtractor("flux-gitrepository", 100),
	}
}

// Matches checks if this extractor applies to Flux GitRepository resources
func (e *FluxGitRepositoryExtractor) Matches(event models.Event) bool {
	return event.Resource.Group == fluxGitRepositoryAPIGroup &&
		event.Resource.Kind == gitRepositoryKind
}

// ExtractRelationships extracts GitRepository→Secret references
func (e *FluxGitRepositoryExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	var gitRepo map[string]interface{}
	if err := json.Unmarshal(event.Data, &gitRepo); err != nil {
		return nil, fmt.Errorf("failed to parse GitRepository: %w", err)
	}

	spec, ok := GetNestedMap(gitRepo, "spec")
	if !ok {
		return edges, nil
	}

	// Extract secretRef (authentication credentials)
	if secretRef, ok := GetNestedMap(spec, "secretRef"); ok {
		if secretName, ok := GetNestedString(secretRef, "name"); ok {
			// Look up the secret
			targetResource, _ := lookup.FindResourceByNamespace(
				ctx,
				event.Resource.Namespace,
				"Secret",
				secretName,
			)
			targetUID := ""
			if targetResource != nil {
				targetUID = targetResource.UID
			}

			edge := e.CreateReferencesSpecEdge(
				event.Resource.UID,
				targetUID,
				"spec.secretRef",
				"Secret",
				secretName,
				event.Resource.Namespace,
			)
			if edge.ToUID != "" {
				edges = append(edges, edge)
			}
		}
	}

	// Extract verify.secretRef (GPG verification)
	if verify, ok := GetNestedMap(spec, "verify"); ok {
		if secretRef, ok := GetNestedMap(verify, "secretRef"); ok {
			if secretName, ok := GetNestedString(secretRef, "name"); ok {
				targetResource, _ := lookup.FindResourceByNamespace(
					ctx,
					event.Resource.Namespace,
					"Secret",
					secretName,
				)
				targetUID := ""
				if targetResource != nil {
					targetUID = targetResource.UID
				}

				edge := e.CreateReferencesSpecEdge(
					event.Resource.UID,
					targetUID,
					"spec.verify.secretRef",
					"Secret",
					secretName,
					event.Resource.Namespace,
				)
				if edge.ToUID != "" {
					edges = append(edges, edge)
				}
			}
		}
	}

	return edges, nil
}
