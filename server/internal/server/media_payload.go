package server

import (
	"encoding/json"
	"strings"
)

// mediaPayload is stored in items.media_json. Older rows contain a plain
// string array of image URLs; current rows use this envelope so podcast audio
// can live next to the image list without a schema migration.
type mediaPayload struct {
	Images    []string `json:"images"`
	AudioURL  string   `json:"audio_url,omitempty"`
	AudioType string   `json:"audio_type,omitempty"`
}

func decodeMediaPayload(raw string) mediaPayload {
	if strings.TrimSpace(raw) == "" {
		return mediaPayload{}
	}
	var payload mediaPayload
	if err := json.Unmarshal([]byte(raw), &payload); err == nil && (payload.Images != nil || payload.AudioURL != "") {
		return payload
	}
	var images []string
	if err := json.Unmarshal([]byte(raw), &images); err == nil {
		return mediaPayload{Images: images}
	}
	return mediaPayload{}
}
