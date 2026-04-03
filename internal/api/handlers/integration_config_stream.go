package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HandleStatusStream handles GET /api/config/integrations/stream - SSE endpoint for real-time status updates.
func (h *IntegrationConfigHandler) HandleStatusStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Error("SSE not supported: ResponseWriter doesn't implement Flusher")
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	h.logger.Debug("SSE client connected for integration status stream")

	lastStatus := make(map[string]string)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	h.sendStatusUpdate(context.Background(), w, flusher, lastStatus)

	for {
		select {
		case <-r.Context().Done():
			h.logger.Debug("SSE client disconnected")
			return
		case <-ticker.C:
			h.sendStatusUpdate(r.Context(), w, flusher, lastStatus)
		}
	}
}

// sendStatusUpdate sends an SSE event if any integration status has changed.
func (h *IntegrationConfigHandler) sendStatusUpdate(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, lastStatus map[string]string) {
	responses, err := h.service.List(ctx)
	if err != nil {
		h.logger.Error("SSE: Failed to load integrations status: %v", err)
		return
	}

	hasChanges := false
	currentNames := make(map[string]bool, len(responses))
	for _, response := range responses {
		currentNames[response.Name] = true
		if lastHealth, exists := lastStatus[response.Name]; !exists || lastHealth != response.Health {
			hasChanges = true
			lastStatus[response.Name] = response.Health
		}
	}

	for name := range lastStatus {
		if !currentNames[name] {
			delete(lastStatus, name)
			hasChanges = true
		}
	}

	if hasChanges || len(lastStatus) == 0 {
		data, err := json.Marshal(responses)
		if err != nil {
			h.logger.Error("SSE: Failed to marshal status: %v", err)
			return
		}

		fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
		flusher.Flush()
	}
}
