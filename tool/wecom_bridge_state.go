package main

import "encoding/json"

func (s *bridgeState) addClient(c *sseClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[c] = struct{}{}
}

func (s *bridgeState) removeClient(c *sseClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, c)
}

func (s *bridgeState) broadcast(payload map[string]any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	s.mu.Lock()
	id := s.nextEventID
	s.nextEventID++
	event := sseEvent{ID: id, Payload: data}
	s.buffer = append(s.buffer, event)
	if len(s.buffer) > s.bufferCap {
		s.buffer = s.buffer[len(s.buffer)-s.bufferCap:]
	}
	for client := range s.clients {
		select {
		case client.ch <- event:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *bridgeState) getMissed(lastEventID int64) []sseEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buffer) == 0 {
		return nil
	}
	missed := make([]sseEvent, 0)
	for _, ev := range s.buffer {
		if ev.ID > lastEventID {
			missed = append(missed, ev)
		}
	}
	return missed
}
