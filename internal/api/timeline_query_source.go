package api

import apptimeline "github.com/moolen/spectre/internal/app/timeline"

func toAppQuerySource(querySource TimelineQuerySource) apptimeline.QuerySource {
	switch querySource {
	case TimelineQuerySourceGraph:
		return apptimeline.QuerySourceGraph
	default:
		return apptimeline.QuerySourceStorage
	}
}
