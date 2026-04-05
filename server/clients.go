package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
)

const paramClientID = "clientID"

func createClient(clients ClientService) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		var cl clientResource
		if err := json.NewDecoder(req.Body).Decode(&cl); err != nil {
			writeJSON(req, wr, http.StatusBadRequest,
				clockwork.ErrInvalid.WithCausef("invalid json body: %v", err))
			return
		}

		createdClient, err := clients.RegisterClient(req.Context(), cl.to())
		if err != nil {
			writeErrJSON(req, wr, err)
			return
		}
		cl.from(*createdClient)

		writeJSON(req, wr, http.StatusCreated, cl)
	})
}

func getClient(clients ClientService) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		id := mux.Vars(req)[paramClientID]

		sc, err := clients.GetClient(req.Context(), id)
		if err != nil {
			writeErrJSON(req, wr, err)
			return
		}

		var resp clientResource
		resp.from(*sc)
		resp.Secret = ""
		writeJSON(req, wr, http.StatusOK, resp)
	})
}

func deleteClient(clients ClientService) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		id := mux.Vars(req)[paramClientID]

		err := clients.DeleteClient(req.Context(), id)
		if err != nil {
			writeErrJSON(req, wr, err)
			return
		}

		writeJSON(req, wr, http.StatusNoContent, nil)
	})
}

type clientResource struct {
	ID          string    `json:"id,omitempty"`
	Secret      string    `json:"secret,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ChannelType string    `json:"channel_type,omitempty"`
	ChannelName string    `json:"channel_name,omitempty"`
}

func (cr clientResource) to() client.Client {
	return client.Client{
		ID:          cr.ID,
		Secret:      cr.Secret,
		CreatedAt:   cr.CreatedAt.UTC(),
		UpdatedAt:   cr.UpdatedAt.UTC(),
		ChannelType: cr.ChannelType,
		ChannelName: cr.ChannelName,
	}
}

func (cr *clientResource) from(c client.Client) {
	*cr = clientResource{
		ID:          c.ID,
		Secret:      c.Secret,
		CreatedAt:   c.CreatedAt.UTC(),
		UpdatedAt:   c.UpdatedAt.UTC(),
		ChannelType: c.ChannelType,
		ChannelName: c.ChannelName,
	}
}
