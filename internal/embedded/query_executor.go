package embedded

import (
	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/models"
)

type QueryExecutor = embeddedstore.QueryExecutor

func NewQueryExecutor(events []models.Event) (*QueryExecutor, error) {
	projection, err := embeddedstore.BuildProjection(events)
	if err != nil {
		return nil, err
	}
	return embeddedstore.NewQueryExecutor(projection), nil
}
