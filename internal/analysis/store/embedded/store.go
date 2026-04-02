package embedded

import (
	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/models"
)

type Store = embeddedstore.Store

func New(events []models.Event) (*Store, error) {
	projection, err := embeddedstore.BuildProjection(events)
	if err != nil {
		return nil, err
	}
	return embeddedstore.NewAnalysisStore(projection), nil
}
