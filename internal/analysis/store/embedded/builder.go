package embedded

import (
	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/models"
)

func buildProjection(events []models.Event) (*embeddedstore.Projection, error) {
	return embeddedstore.BuildProjection(events)
}
