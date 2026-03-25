package eventbus

import (
	"sync"

	"github.com/chentianyu/celestia/internal/models"
)

type Bus struct {
	mu          sync.RWMutex
	subscribers map[int]chan models.Event
	nextID      int
}

func New() *Bus {
	return &Bus{
		subscribers: make(map[int]chan models.Event),
	}
}

func (b *Bus) Publish(event models.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *Bus) Subscribe(buffer int) (int, <-chan models.Event) {
	if buffer <= 0 {
		buffer = 32
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextID
	b.nextID++
	ch := make(chan models.Event, buffer)
	b.subscribers[id] = ch
	return id, ch
}

func (b *Bus) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch, ok := b.subscribers[id]
	if !ok {
		return
	}
	delete(b.subscribers, id)
	close(ch)
}

