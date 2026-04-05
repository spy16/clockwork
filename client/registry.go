package client

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/spy16/clockwork"
)

func NewRegistry(store Store, admins []string, channelNames []string) *Registry {
	return &Registry{
		Store:    store,
		Admins:   removeEmpty(admins),
		Channels: removeEmpty(channelNames),
	}
}

// Registry provides functions to manage clients.
type Registry struct {
	Store    Store
	Admins   []string
	Channels []string
}

// IsAdmin returns true if the given client is whitelisted as admin.
func (tim *Registry) IsAdmin(_ context.Context, id string) bool {
	id = strings.TrimSpace(id)
	return slices.Contains(tim.Admins, id)
}

// GetClient returns client by ID. If not found, returns errors.ErrNotFound.
func (tim *Registry) GetClient(ctx context.Context, id string) (*Client, error) {
	return tim.Store.Get(ctx, id)
}

// DeleteClient deletes client by ID. Returns success even if client is not
// found.
func (tim *Registry) DeleteClient(ctx context.Context, id string) error {
	cl := From(ctx)
	if cl == nil || (!tim.IsAdmin(ctx, cl.ID) && id != cl.ID) {
		return clockwork.ErrUnauthorized.WithMsgf("only an admin or the client itself can delete")
	}
	err := tim.Store.Del(ctx, id)
	if err != nil && !errors.Is(err, clockwork.ErrNotFound) {
		return err
	}
	return nil
}

// RegisterClient registers a new client with given config. If the client id
// and/or client secret are empty, they will be assigned automatically.
func (tim *Registry) RegisterClient(ctx context.Context, cl Client) (*Client, error) {
	if !contains(tim.Channels, cl.ChannelType) {
		return nil, clockwork.ErrInvalid.WithCausef("unsupported channel type: %s", cl.ChannelType)
	}

	plainSecret, err := cl.GenerateCreds()
	if err != nil {
		return nil, err
	}

	if err := cl.Validate(); err != nil {
		return nil, err
	}

	if err := tim.Store.Put(ctx, cl); err != nil {
		return nil, err
	}

	// Return plain secret so that user can make a note of it.
	cl.Secret = plainSecret
	return &cl, nil
}

func contains(arr []string, item string) bool {
	return slices.Contains(arr, item)
}

func removeEmpty(arr []string) []string {
	var result []string
	for _, item := range arr {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
