package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

type channelTextsReq struct {
	ChannelType string   `json:"channelType"`
	Language    string   `json:"language"`
	Keys        []string `json:"keys"`
}

type channelTextItem struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

type channelTextsData struct {
	ChannelType string            `json:"channelType"`
	Language    string            `json:"language"`
	Items       []channelTextItem `json:"items"`
}

func (s *Server) handleChannelTexts(w http.ResponseWriter, r *http.Request) {
	var req channelTextsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{
			Code:    1400,
			Message: "invalid request body",
		})
		return
	}
	if len(req.Keys) == 0 {
		s.writeJSON(w, http.StatusBadRequest, Response{
			Code:    1400,
			Message: "keys is required",
		})
		return
	}

	items, err := s.channelTextService.Resolve(req.ChannelType, req.Language, req.Keys)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{
			Code:    2401,
			Message: "load channel texts failed: " + err.Error(),
		})
		return
	}

	respItems := make([]channelTextItem, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.Key) == "" {
			continue
		}
		respItems = append(respItems, channelTextItem{
			Key:  it.Key,
			Text: it.Text,
		})
	}

	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: channelTextsData{
			ChannelType: req.ChannelType,
			Language:    req.Language,
			Items:       respItems,
		},
	})
}
