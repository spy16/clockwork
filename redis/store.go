package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v8"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
)

var _ client.Store = (*ClientStore)(nil)

type ClientStore struct {
	Client redis.UniversalClient
}

func (store *ClientStore) Get(ctx context.Context, id string) (*client.Client, error) {
	val, err := store.Client.Get(ctx, clientKey(id)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, clockwork.ErrNotFound
		}
		return nil, err
	}
	var cl client.Client
	if err := json.Unmarshal([]byte(val), &cl); err != nil {
		return nil, err
	}
	return &cl, nil
}

func (store *ClientStore) Put(ctx context.Context, cl client.Client) error {
	if err := cl.Validate(); err != nil {
		return err
	}

	isOk, err := store.Client.SetNX(ctx, clientKey(cl.ID), jsonBytes(cl), 0).Result()
	if err != nil {
		return err
	} else if !isOk {
		return clockwork.ErrConflict.WithMsgf("client with given id already exists")
	}
	return nil
}

func (store *ClientStore) Del(ctx context.Context, id string) error {
	val, err := store.Client.Del(ctx, clientKey(id)).Result()
	if err != nil {
		if err == redis.Nil {
			return clockwork.ErrNotFound
		}
		return err
	} else if val == 0 {
		return clockwork.ErrNotFound
	}
	return nil
}

func clientKey(id string) string {
	// WARNING: changing this key will cause lookups for ALL existing clients in
	// clockwork deployments to fail.
	return fmt.Sprintf("clockwork:client:{%s}", id)
}
