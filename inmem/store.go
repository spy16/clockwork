package inmem

import (
	"context"
	"sync"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
)

var _ client.Store = (*ClientStore)(nil)

type ClientStore struct {
	mu      sync.RWMutex
	clients map[string]client.Client
}

func (store *ClientStore) Get(_ context.Context, id string) (*client.Client, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	cl, found := store.clients[id]
	if !found {
		return nil, clockwork.ErrNotFound.WithCausef("no client with id '%s'", id)
	}
	return &cl, nil
}

func (store *ClientStore) Put(_ context.Context, cl client.Client) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	_, alreadyExists := store.clients[cl.ID]
	if alreadyExists {
		return clockwork.ErrConflict
	}

	if store.clients == nil {
		store.clients = map[string]client.Client{}
	}
	store.clients[cl.ID] = cl
	return nil
}

func (store *ClientStore) Del(_ context.Context, id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	_, found := store.clients[id]
	if !found {
		return clockwork.ErrNotFound
	}

	delete(store.clients, id)
	return nil
}
