package inmem_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/inmem"
	"github.com/stretchr/testify/assert"
)

func TestClientStore_Get(t *testing.T) {
	t.Parallel()

	t.Run("ClientNotFound", func(t *testing.T) {
		st := &inmem.ClientStore{}
		cl, err := st.Get(context.Background(), "foo")
		assert.True(t, errors.Is(err, clockwork.ErrNotFound))
		assert.Nil(t, cl)
	})

	t.Run("ClientFound", func(t *testing.T) {
		st := &inmem.ClientStore{}
		err := st.Put(context.Background(), client.Client{ID: "foo"})
		assert.NoError(t, err)

		cl, err := st.Get(context.Background(), "foo")
		assert.NoError(t, err)
		assert.NotNil(t, cl)
		assert.Equal(t, "foo", cl.ID)
	})
}

func TestClientStore_Put(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		st := &inmem.ClientStore{}
		cl, err := st.Get(context.Background(), "foo")
		assert.True(t, errors.Is(err, clockwork.ErrNotFound))
		assert.Nil(t, cl)
	})

	t.Run("ClientAlreadyExists", func(t *testing.T) {
		st := &inmem.ClientStore{}
		err := st.Put(context.Background(), client.Client{ID: "foo"})
		assert.NoError(t, err)

		err = st.Put(context.Background(), client.Client{ID: "foo"})
		assert.True(t, errors.Is(err, clockwork.ErrConflict), "expected ErrConflict, got %v", err)
	})
}

func TestClientStore_Del(t *testing.T) {
	t.Parallel()

	t.Run("ClientNotFound", func(t *testing.T) {
		st := &inmem.ClientStore{}
		err := st.Del(context.Background(), "foo")
		assert.True(t, errors.Is(err, clockwork.ErrNotFound))
	})

	t.Run("ClientFound", func(t *testing.T) {
		st := &inmem.ClientStore{}
		err := st.Put(context.Background(), client.Client{ID: "foo"})
		assert.NoError(t, err)

		err = st.Del(context.Background(), "foo")
		assert.NoError(t, err)

		cl, err := st.Get(context.Background(), "foo")
		assert.True(t, errors.Is(err, clockwork.ErrNotFound))
		assert.Nil(t, cl)
	})
}
