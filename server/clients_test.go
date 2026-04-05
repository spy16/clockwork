package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/server"
	"github.com/spy16/clockwork/server/mocks"
	"github.com/stretchr/testify/mock"
)

var frozenTime = time.Unix(1645509802, 0).UTC()

func Test_createClient(t *testing.T) {
	runAll(t, []endpointTestCase{
		{
			title: "InvalidJSON",
			request: httpReq{
				Method: http.MethodPost,
				Path:   "/v1/clients",
				Body:   `invalid json body`,
			},
			want: httpResp{
				Status: 400,
				Body: `{
						"code":"bad_request",
						"message":"Request is not valid",
						"cause":"invalid json body: invalid character 'i' looking for beginning of value"
					}`,
			},
		},
		{
			title: "InvalidRequest",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					RegisterClient(mock.Anything, mock.Anything).
					Return(nil, clockwork.ErrInvalid.WithCausef("id must not be empty")).
					Once()

				scheduleSvc := &mocks.ScheduleService{}

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method: http.MethodPost,
				Path:   "/v1/clients",
				Body:   `{}`,
			},
			want: httpResp{
				Status: 400,
				Body: `{
						"code":"bad_request",
						"message":"Request is not valid",
						"cause":"id must not be empty"
					}`,
			},
		},
		{
			title: "Successful",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					RegisterClient(mock.Anything, client.Client{ID: "foo", ChannelType: "log"}).
					Return(&client.Client{
						ID:          "foo",
						Secret:      "sample",
						CreatedAt:   frozenTime,
						UpdatedAt:   frozenTime,
						ChannelType: "log",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method: http.MethodPost,
				Path:   "/v1/clients",
				Body:   `{"id": "foo", "channel_type": "log"}`,
			},
			want: httpResp{
				Status: 201,
				Body: `{
						"id":"foo", 
						"secret":"sample", 
						"channel_type":"log", 
						"created_at":"2022-02-22T06:03:22Z", 
						"updated_at":"2022-02-22T06:03:22Z"
					}`,
			},
		},
	})
}

func Test_getClient(t *testing.T) {
	runAll(t, []endpointTestCase{
		{
			title: "NotFound",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(nil, clockwork.ErrNotFound.WithMsgf("no client with id")).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method: http.MethodGet,
				Path:   "/v1/clients/foo",
			},
			want: httpResp{
				Status: 404,
				Body:   `{"code":"not_found", "message":"no client with id"}`,
			},
		},
		{
			title: "Successful",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID:          "foo",
						Secret:      "secret-hash",
						CreatedAt:   frozenTime,
						UpdatedAt:   frozenTime,
						ChannelName: "foo-bar",
						ChannelType: "log",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method: http.MethodGet,
				Path:   "/v1/clients/foo",
			},
			want: httpResp{
				Status: 200,
				Body: `{
					"id":"foo", 
					"channel_name":"foo-bar", 
					"channel_type":"log", 
					"created_at":"2022-02-22T06:03:22Z", 
					"updated_at":"2022-02-22T06:03:22Z"
				}`,
			},
		},
	})
}

func Test_deleteClient(t *testing.T) {
	runAll(t, []endpointTestCase{
		{
			title: "SomeError",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).Once()

				clientSvc.EXPECT().
					DeleteClient(mock.Anything, "foo").
					Return(clockwork.ErrInternal).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method: http.MethodDelete,
				Path:   "/v1/clients/foo",
				Headers: http.Header{
					"Authorization": []string{"Basic Zm9vOmJhcg=="},
				},
			},
			want: httpResp{
				Status: 500,
				Body:   `{"code":"internal_error", "message":"Some unexpected error occurred"}`,
			},
		},
		{
			title: "Successful",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).Once()

				clientSvc.EXPECT().
					DeleteClient(mock.Anything, "foo").
					Return(nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method: http.MethodDelete,
				Path:   "/v1/clients/foo",
				Headers: http.Header{
					"Authorization": []string{"Basic Zm9vOmJhcg=="},
				},
			},
			want: httpResp{
				Status: 204,
			},
		},
	})
}
