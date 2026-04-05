package client_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
	"github.com/stretchr/testify/assert"
)

func TestRegistry_IsAdmin(t *testing.T) {
	reg := client.NewRegistry(nil, []string{"foo", " bar ", "", "       "}, nil)

	assert.True(t, reg.IsAdmin(context.Background(), "foo"))
	assert.True(t, reg.IsAdmin(context.Background(), "         foo          "))

	assert.False(t, reg.IsAdmin(context.Background(), "foo bar"))
	assert.False(t, reg.IsAdmin(context.Background(), "/bar"))
	assert.False(t, reg.IsAdmin(context.Background(), ""))
}

func TestRegistry_RegisterClient(t *testing.T) {
	t.Parallel()

	validCl := client.Client{
		ChannelType: "log",
		ChannelName: "foo",
	}

	t.Run("UnsupportedChannel", func(t *testing.T) {
		service := &client.Registry{Channels: []string{"log"}}
		cl, err := service.RegisterClient(context.Background(), client.Client{
			ChannelType: "unsupported-channel-type",
		})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, clockwork.ErrInvalid))
		assert.Nil(t, cl)
	})

	t.Run("ValidationErr", func(t *testing.T) {
		service := &client.Registry{Channels: []string{"log"}}
		cl, err := service.RegisterClient(context.Background(), client.Client{
			ChannelType: "log",
		})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, clockwork.ErrInvalid))
		assert.Nil(t, cl)
	})

	t.Run("StoreError", func(t *testing.T) {
		service := &client.Registry{
			Channels: []string{"log"},
			Store: &fakeStore{
				PutFunc: func(ctx context.Context, cl client.Client) error {
					return clockwork.Errorf("failed")
				},
			},
		}

		cl, err := service.RegisterClient(context.Background(), validCl)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, clockwork.ErrInternal))
		assert.Nil(t, cl)
	})

	t.Run("Success", func(t *testing.T) {
		service := &client.Registry{
			Channels: []string{"log"},
			Store: &fakeStore{
				PutFunc: func(ctx context.Context, cl client.Client) error {
					assert.NotEmpty(t, cl.ID)
					assert.NotEmpty(t, cl.Secret)
					return nil
				},
			},
		}

		cl, err := service.RegisterClient(context.Background(), validCl)
		assert.NoError(t, err)
		assert.NotNil(t, cl)
	})
}

func TestRegistry_DeleteClient(t *testing.T) {
	t.Parallel()

	ctx := client.Context(context.Background(), client.Client{ID: "foo"})

	t.Run("Unauthorized", func(t *testing.T) {
		service := &client.Registry{
			Store: &fakeStore{
				DelFunc: func(ctx context.Context, id string) error {
					return errors.New("failed")
				},
			},
		}
		err := service.DeleteClient(ctx, "del")
		assert.Error(t, err)
	})

	t.Run("UnknownError", func(t *testing.T) {
		service := &client.Registry{
			Store: &fakeStore{
				DelFunc: func(ctx context.Context, id string) error {
					return errors.New("failed")
				},
			},
		}
		err := service.DeleteClient(ctx, "foo")
		assert.Error(t, err)
	})

	t.Run("NotFoundError", func(t *testing.T) {
		service := &client.Registry{
			Store: &fakeStore{
				DelFunc: func(ctx context.Context, id string) error {
					return clockwork.ErrNotFound
				},
			},
		}
		err := service.DeleteClient(ctx, "foo")
		assert.NoError(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		service := &client.Registry{
			Store: &fakeStore{
				DelFunc: func(ctx context.Context, id string) error {
					return clockwork.ErrNotFound
				},
			},
		}
		err := service.DeleteClient(ctx, "foo")
		assert.NoError(t, err)
	})
}

func TestRegistry_GetClient(t *testing.T) {
	t.Parallel()

	t.Run("ClientNotFound", func(t *testing.T) {
		service := &client.Registry{
			Store: &fakeStore{
				GetFunc: func(ctx context.Context, id string) (*client.Client, error) {
					return nil, clockwork.ErrNotFound
				},
			},
		}
		cl, err := service.GetClient(context.Background(), "foo")
		assert.Error(t, err)
		assert.Nil(t, cl)
	})

	t.Run("Success", func(t *testing.T) {
		service := &client.Registry{
			Store: &fakeStore{
				GetFunc: func(ctx context.Context, id string) (*client.Client, error) {
					return &client.Client{ID: id}, nil
				},
			},
		}
		cl, err := service.GetClient(context.Background(), "foo")
		assert.NoError(t, err)
		assert.NotNil(t, cl)
	})
}

type fakeStore struct {
	GetFunc func(ctx context.Context, id string) (*client.Client, error)
	PutFunc func(ctx context.Context, cl client.Client) error
	DelFunc func(ctx context.Context, id string) error
}

func (fs *fakeStore) Get(ctx context.Context, id string) (*client.Client, error) {
	return fs.GetFunc(ctx, id)
}
func (fs *fakeStore) Put(ctx context.Context, cl client.Client) error { return fs.PutFunc(ctx, cl) }
func (fs *fakeStore) Del(ctx context.Context, id string) error        { return fs.DelFunc(ctx, id) }
