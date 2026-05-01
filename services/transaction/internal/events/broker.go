package events

import (
	"context"
	"sync"

	"github.com/example/fintech-core-api/services/transaction/internal/domain"
)

type Broker struct {
	mu      sync.RWMutex
	clients map[chan domain.TransactionEvent]struct{}
}

func NewBroker() *Broker {
	return &Broker{clients: make(map[chan domain.TransactionEvent]struct{})}
}

func (b *Broker) Publish(event domain.TransactionEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for client := range b.clients {
		select {
		case client <- event:
		default:
		}
	}
}

func (b *Broker) Subscribe(ctx context.Context) <-chan domain.TransactionEvent {
	client := make(chan domain.TransactionEvent, 16)
	b.mu.Lock()
	b.clients[client] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.clients, client)
		close(client)
		b.mu.Unlock()
	}()

	return client
}
