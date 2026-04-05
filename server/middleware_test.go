package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/server/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_panicRecovery(t *testing.T) {
	mw := panicRecovery()

	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("helo")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"code":"internal_error", "message":"Some unexpected error occurred"}`, w.Body.String())
}

func Test_clientAuth(t *testing.T) {
	t.Run("NoVerify", func(t *testing.T) {
		clientSvc := &mocks.ClientService{}

		mw := clientAuth(clientSvc, false)

		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("response from handler"))
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		h.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.JSONEq(t, `{
			"code": "unauthorized",
			"cause": "authorization header must be specified",
			"message": "Client is not authorized for the requested action"
		}`, w.Body.String())
	})

	t.Run("WithVerify_InvalidClient", func(t *testing.T) {
		clientSvc := &mocks.ClientService{}
		clientSvc.EXPECT().GetClient(mock.Anything, "foo").Return(nil, clockwork.ErrNotFound).Once()

		mw := clientAuth(clientSvc, false)

		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("response from handler"))
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header = http.Header{
			"Authorization": {"Basic Zm9vOmJhcg=="},
		}

		h.ServeHTTP(w, r)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.JSONEq(t, `{
			"code": "unauthorized",
			"cause": "client id or secret is not valid",
			"message": "Client is not authorized for the requested action"
		}`, w.Body.String())
	})

	t.Run("WithVerify_ValidClient", func(t *testing.T) {
		clientSvc := &mocks.ClientService{}
		clientSvc.EXPECT().GetClient(mock.Anything, "foo").Return(&client.Client{
			ID: "foo",
			// BCrypt hash of secret 'bar'
			Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
		}, nil).Once()

		mw := clientAuth(clientSvc, false)

		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("response from handler"))
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header = http.Header{
			"Authorization": {"Basic Zm9vOmJhcg=="},
		}

		h.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, `response from handler`, w.Body.String())
	})
}
