package realtime

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

// One hub; subscribers keyed by project id.
type Hub struct {
	mu   sync.RWMutex
	subs map[uuid.UUID][]chan []byte
}

func NewHub() *Hub {
	return &Hub{subs: make(map[uuid.UUID][]chan []byte)}
}

func (h *Hub) Subscribe(projectID uuid.UUID) (ch <-chan []byte, cancel func()) {
	c := make(chan []byte, 16)
	h.mu.Lock()
	h.subs[projectID] = append(h.subs[projectID], c)
	h.mu.Unlock()
	return c, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		list := h.subs[projectID]
		for i, x := range list {
			if x == c {
				h.subs[projectID] = append(list[:i], list[i+1:]...)
				close(c)
				break
			}
		}
		if len(h.subs[projectID]) == 0 {
			delete(h.subs, projectID)
		}
	}
}

func (h *Hub) PublishProjectTasks(projectID uuid.UUID) {
	payload, err := json.Marshal(map[string]string{
		"type":       "project_tasks_changed",
		"project_id": projectID.String(),
	})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.subs[projectID] {
		select {
		case c <- payload:
		default:
			// slow client - skip
		}
	}
}
