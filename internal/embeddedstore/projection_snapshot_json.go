package embeddedstore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

type projectionResourceVersionSnapshotJSON struct {
	EventID     string                        `json:"event_id"`
	Timestamp   int64                         `json:"timestamp"`
	EventType   models.EventType              `json:"event_type"`
	Identity    graph.ResourceIdentity        `json:"identity"`
	Data        json.RawMessage               `json:"data,omitempty"`
	DataBase64  string                        `json:"data_base64,omitempty"`
	ChangeEvent analysisstore.ChangeEventInfo `json:"change_event"`
}

func (s ProjectionResourceVersionSnapshot) MarshalJSON() ([]byte, error) {
	encoded := projectionResourceVersionSnapshotJSON{
		EventID:     s.EventID,
		Timestamp:   s.Timestamp,
		EventType:   s.EventType,
		Identity:    s.Identity,
		ChangeEvent: s.ChangeEvent,
	}
	if len(s.Data) > 0 {
		if json.Valid(s.Data) {
			encoded.Data = json.RawMessage(s.Data)
		} else {
			encoded.DataBase64 = base64.StdEncoding.EncodeToString(s.Data)
		}
	}
	return json.Marshal(encoded)
}

func (s *ProjectionResourceVersionSnapshot) UnmarshalJSON(payload []byte) error {
	var decoded projectionResourceVersionSnapshotJSON
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return err
	}

	*s = ProjectionResourceVersionSnapshot{
		EventID:     decoded.EventID,
		Timestamp:   decoded.Timestamp,
		EventType:   decoded.EventType,
		Identity:    decoded.Identity,
		ChangeEvent: decoded.ChangeEvent,
	}

	switch {
	case decoded.DataBase64 != "":
		data, err := base64.StdEncoding.DecodeString(decoded.DataBase64)
		if err != nil {
			return fmt.Errorf("decode data_base64: %w", err)
		}
		s.Data = data
	case len(decoded.Data) == 0 || string(decoded.Data) == "null":
		s.Data = nil
	case decoded.Data[0] == '"':
		var legacyBase64 string
		if err := json.Unmarshal(decoded.Data, &legacyBase64); err != nil {
			return fmt.Errorf("decode legacy base64 payload: %w", err)
		}
		data, err := base64.StdEncoding.DecodeString(legacyBase64)
		if err != nil {
			return fmt.Errorf("decode legacy base64 payload: %w", err)
		}
		s.Data = data
	default:
		s.Data = append([]byte(nil), decoded.Data...)
	}

	return nil
}
