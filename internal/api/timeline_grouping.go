package api

import (
	"sort"

	"github.com/moolen/spectre/internal/models"
)

// GroupedResources groups resources by kind
type GroupedResources struct {
	Kind      string
	Resources []models.Resource
}

// groupResourcesByKind groups resources by kind and sorts kinds alphabetically
func groupResourcesByKind(resources []models.Resource) []*GroupedResources {
	groups := make(map[string][]models.Resource)
	
	for _, r := range resources {
		groups[r.Kind] = append(groups[r.Kind], r)
	}
	
	// Convert to slice and sort kinds alphabetically
	kinds := make([]string, 0, len(groups))
	for k := range groups {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	
	result := make([]*GroupedResources, 0, len(groups))
	for _, kind := range kinds {
		result = append(result, &GroupedResources{
			Kind:      kind,
			Resources: groups[kind],
		})
	}
	
	return result
}

// sortResourcesByNamespaceAndName sorts resources within a group by namespace, then name
func sortResourcesByNamespaceAndName(resources []models.Resource) {
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Namespace != resources[j].Namespace {
			return resources[i].Namespace < resources[j].Namespace
		}
		return resources[i].Name < resources[j].Name
	})
}

// groupAndSortResources groups resources by kind and sorts them within each group
func groupAndSortResources(resources []models.Resource) []*GroupedResources {
	grouped := groupResourcesByKind(resources)
	
	// Sort resources within each kind group
	for _, group := range grouped {
		sortResourcesByNamespaceAndName(group.Resources)
	}
	
	return grouped
}
