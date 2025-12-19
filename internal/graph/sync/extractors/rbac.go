package extractors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// RBACExtractor extracts RBAC relationships from Role/RoleBinding/ClusterRole/ClusterRoleBinding resources
type RBACExtractor struct {
	logger *logging.Logger
}

// NewRBACExtractor creates a new RBAC extractor
func NewRBACExtractor() *RBACExtractor {
	return &RBACExtractor{
		logger: logging.GetLogger("extractors.rbac"),
	}
}

func (e *RBACExtractor) Name() string {
	return "rbac"
}

func (e *RBACExtractor) Priority() int {
	return 50 // Run between native K8s (0-99) and custom resources (100+)
}

func (e *RBACExtractor) Matches(event models.Event) bool {
	// Match RoleBinding and ClusterRoleBinding resources
	return (event.Resource.Group == "rbac.authorization.k8s.io" &&
		(event.Resource.Kind == "RoleBinding" || event.Resource.Kind == "ClusterRoleBinding"))
}

func (e *RBACExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	// Parse the RoleBinding/ClusterRoleBinding
	var binding map[string]interface{}
	if err := json.Unmarshal(event.Data, &binding); err != nil {
		return nil, fmt.Errorf("failed to parse binding: %w", err)
	}

	// Extract roleRef
	roleRef, ok := binding["roleRef"].(map[string]interface{})
	if !ok {
		e.logger.Debug("%s %s/%s has no roleRef", event.Resource.Kind, event.Resource.Namespace, event.Resource.Name)
		return edges, nil
	}

	// Extract role information
	roleKind, _ := roleRef["kind"].(string)
	roleName, _ := roleRef["name"].(string)

	if roleKind == "" || roleName == "" {
		e.logger.Debug("%s %s/%s has invalid roleRef", event.Resource.Kind, event.Resource.Namespace, event.Resource.Name)
		return edges, nil
	}

	// Find the Role/ClusterRole resource
	roleNamespace := event.Resource.Namespace
	if roleKind == "ClusterRole" {
		roleNamespace = "" // ClusterRoles are cluster-scoped
	}

	roleResource, err := lookup.FindResourceByNamespace(ctx, roleNamespace, roleKind, roleName)
	if err != nil {
		e.logger.Debug("Role %s/%s not found in graph yet, skipping BINDS_ROLE edge", roleNamespace, roleName)
	} else if roleResource != nil {
		// Create BINDS_ROLE edge
		edge := e.createBindsRoleEdge(event.Resource.UID, roleResource.UID, roleKind, roleName)
		edges = append(edges, edge)
		e.logger.Debug("Created BINDS_ROLE edge: %s → %s (%s)", event.Resource.Name, roleName, roleKind)
	}

	// Extract subjects
	subjects, ok := binding["subjects"].([]interface{})
	if !ok || len(subjects) == 0 {
		e.logger.Debug("%s %s/%s has no subjects", event.Resource.Kind, event.Resource.Namespace, event.Resource.Name)
		return edges, nil
	}

	// Process each subject
	for _, subjectRaw := range subjects {
		subject, ok := subjectRaw.(map[string]interface{})
		if !ok {
			continue
		}

		subjectKind, _ := subject["kind"].(string)
		subjectName, _ := subject["name"].(string)
		subjectNamespace, _ := subject["namespace"].(string)

		// Default namespace for ServiceAccount subjects
		if subjectKind == "ServiceAccount" && subjectNamespace == "" {
			subjectNamespace = event.Resource.Namespace
		}

		if subjectKind == "" || subjectName == "" {
			continue
		}

		// Only create edges for ServiceAccounts (we don't track User/Group in the graph)
		if subjectKind == "ServiceAccount" {
			subjectResource, err := lookup.FindResourceByNamespace(ctx, subjectNamespace, "ServiceAccount", subjectName)
			if err != nil {
				e.logger.Debug("ServiceAccount %s/%s not found in graph yet, skipping GRANTS_TO edge", subjectNamespace, subjectName)
			} else if subjectResource != nil {
				// Create GRANTS_TO edge
				edge := e.createGrantsToEdge(event.Resource.UID, subjectResource.UID, subjectKind, subjectName, subjectNamespace)
				edges = append(edges, edge)
				e.logger.Debug("Created GRANTS_TO edge: %s → ServiceAccount %s/%s", event.Resource.Name, subjectNamespace, subjectName)
			}
		}
	}

	return edges, nil
}

// createBindsRoleEdge creates a BINDS_ROLE edge
func (e *RBACExtractor) createBindsRoleEdge(bindingUID, roleUID, roleKind, roleName string) graph.Edge {
	props := graph.BindsRoleEdge{
		RoleKind: roleKind,
		RoleName: roleName,
	}

	propsJSON, _ := json.Marshal(props)

	return graph.Edge{
		Type:       graph.EdgeTypeBindsRole,
		FromUID:    bindingUID,
		ToUID:      roleUID,
		Properties: propsJSON,
	}
}

// createGrantsToEdge creates a GRANTS_TO edge
func (e *RBACExtractor) createGrantsToEdge(bindingUID, subjectUID, subjectKind, subjectName, subjectNamespace string) graph.Edge {
	props := graph.GrantsToEdge{
		SubjectKind:      subjectKind,
		SubjectName:      subjectName,
		SubjectNamespace: subjectNamespace,
	}

	propsJSON, _ := json.Marshal(props)

	return graph.Edge{
		Type:       graph.EdgeTypeGrantsTo,
		FromUID:    bindingUID,
		ToUID:      subjectUID,
		Properties: propsJSON,
	}
}
