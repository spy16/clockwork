package server_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/schedule"
	"github.com/spy16/clockwork/server"
	"github.com/spy16/clockwork/server/mocks"
	"github.com/stretchr/testify/mock"
)

var (
	sampleSchedule = schedule.Schedule{
		ID:              "s1",
		Tags:            []string{"team:foo"},
		Status:          "ACTIVE",
		Crontab:         "@every 1h",
		Triggers:        nil,
		Version:         1,
		Category:        "foo",
		Payload:         "{}",
		ClientID:        "foo",
		CreatedAt:       frozenTime,
		UpdatedAt:       frozenTime,
		EnqueueCount:    1,
		NextExecutionAt: frozenTime,
	}

	defaultHeaders = http.Header{
		"Authorization": []string{"Basic Zm9vOmJhcg=="},
	}
)

func Test_getSchedule(t *testing.T) {
	runAll(t, []endpointTestCase{
		{
			title: "AuthorizationError",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				scheduleSvc := &mocks.ScheduleService{}
				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method: http.MethodGet,
				Path:   "/v1/schedules/foo",
			},
			want: httpResp{
				Status: 403,
				Body: `{
					"code": "unauthorized", 
					"cause": "authorization header must be specified", 
					"message": "Client is not authorized for the requested action"
				}`,
			},
		},
		{
			title: "UnexpectedError",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					Fetch(mock.Anything, "foo").
					Return(nil, errors.New("totally unexpected")).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodGet,
				Path:    "/v1/schedules/foo",
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 500,
				Body: `{
					"code": "internal_error", 
					"cause": "totally unexpected", 
					"message": "Some unexpected error occurred"
				}`,
			},
		},
		{
			title: "NotFound",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					Fetch(mock.Anything, "foo").
					Return(nil, clockwork.ErrNotFound.WithMsgf("schedule not found")).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodGet,
				Path:    "/v1/schedules/foo",
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 404,
				Body:   `{"code":"not_found", "message":"schedule not found"}`,
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
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					Fetch(mock.Anything, "foo").
					Return(&sampleSchedule, nil).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodGet,
				Path:    "/v1/schedules/foo",
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 200,
				Body: jsonBody(map[string]any{
					"id":               "s1",
					"tags":             []string{"team:foo"},
					"status":           "ACTIVE",
					"crontab":          "@every 1h",
					"version":          1,
					"payload":          "{}",
					"category":         "foo",
					"client_id":        "foo",
					"created_at":       "2022-02-22T06:03:22Z",
					"updated_at":       "2022-02-22T06:03:22Z",
					"enqueue_count":    1,
					"last_enqueued_at": "2022-02-22T06:03:22Z",
				}),
			},
		},
	})
}

func Test_listSchedules(t *testing.T) {
	runAll(t, []endpointTestCase{
		{
			title: "AuthorizationError",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				scheduleSvc := &mocks.ScheduleService{}

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method: http.MethodGet,
				Path:   "/v1/schedules",
			},
			want: httpResp{
				Status: 403,
				Body: `{
					"code": "unauthorized",
					"cause": "authorization header must be specified",
					"message": "Client is not authorized for the requested action"
				}`,
			},
		},
		{
			title: "UnexpectedError",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil)

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					List(mock.Anything, 0, -1).
					Return(nil, clockwork.ErrInternal.WithCausef("failed to list"))

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodGet,
				Path:    "/v1/schedules",
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 500,
				Body:   `{"cause":"failed to list", "code":"internal_error", "message":"Some unexpected error occurred"}`,
			},
		},
		{
			title: "Unsupported",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil)

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					List(mock.Anything, 0, -1).
					Return(nil, clockwork.ErrUnsupported)

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodGet,
				Path:    "/v1/schedules",
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 422,
				Body:   `{"code":"unsupported", "message":"Requested feature is not supported"}`,
			},
		},
		{
			title: "Empty",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					List(mock.Anything, 0, -1).
					Return(nil, nil).
					Once()
				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodGet,
				Path:    "/v1/schedules",
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 200,
				Body:   `[]`,
			},
		},
		{
			title: "SchedulesFound",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					List(mock.Anything, 0, -1).
					Return([]schedule.Schedule{sampleSchedule}, nil).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodGet,
				Path:    "/v1/schedules",
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 200,
				Body: jsonBody([]any{
					map[string]any{
						"id":               "s1",
						"tags":             []string{"team:foo"},
						"status":           "ACTIVE",
						"crontab":          "@every 1h",
						"version":          1,
						"payload":          "{}",
						"category":         "foo",
						"client_id":        "foo",
						"created_at":       "2022-02-22T06:03:22Z",
						"updated_at":       "2022-02-22T06:03:22Z",
						"enqueue_count":    1,
						"last_enqueued_at": "2022-02-22T06:03:22Z",
					},
				}),
			},
		},
	})
}

func Test_createSchedule(t *testing.T) {
	runAll(t, []endpointTestCase{
		{
			title: "AuthorizationError",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				scheduleSvc := &mocks.ScheduleService{}

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method: http.MethodPost,
				Path:   "/v1/schedules",
			},
			want: httpResp{
				Status: 403,
				Body: `{
					"code": "unauthorized", 
					"cause": "authorization header must be specified", 
					"message": "Client is not authorized for the requested action"
				}`,
			},
		},
		{
			title: "InvalidBody",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodPost,
				Path:    "/v1/schedules",
				Body:    ``,
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 400,
				Body: `{
					"code": "bad_request",
					"cause": "invalid json body: EOF", 
					"message": "Request is not valid"
				}`,
			},
		},
		{
			title: "InvalidSchedule",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					Create(mock.Anything, schedule.Schedule{ClientID: "foo"}).
					Return(nil, clockwork.ErrInvalid.WithCausef("id must not be empty")).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodPost,
				Path:    "/v1/schedules",
				Body:    `{}`,
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 400,
				Body: `{
					"code": "bad_request",
					"cause": "id must not be empty",
					"message": "Request is not valid"
				}`,
			},
		},
		{
			title: "Conflict",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					Create(mock.Anything, sampleSchedule).
					Return(nil, clockwork.ErrConflict.WithMsgf("already exists")).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodPost,
				Path:    "/v1/schedules",
				Body:    jsonBody(sampleSchedule),
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 409,
				Body:   `{"code":"conflict", "message":"already exists"}`,
			},
		},
		{
			title: "Success",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					Create(mock.Anything, sampleSchedule).
					Return(&sampleSchedule, nil).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodPost,
				Path:    "/v1/schedules",
				Body:    jsonBody(sampleSchedule),
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 201,
				Body: jsonBody(map[string]any{
					"id":               "s1",
					"tags":             []string{"team:foo"},
					"status":           "ACTIVE",
					"crontab":          "@every 1h",
					"version":          1,
					"payload":          "{}",
					"category":         "foo",
					"client_id":        "foo",
					"created_at":       "2022-02-22T06:03:22Z",
					"updated_at":       "2022-02-22T06:03:22Z",
					"enqueue_count":    1,
					"last_enqueued_at": "2022-02-22T06:03:22Z",
				}),
			},
		},
	})
}

func Test_updateSchedule(t *testing.T) {
	runAll(t, []endpointTestCase{
		{
			title: "AuthorizationError",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				scheduleSvc := &mocks.ScheduleService{}

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method: http.MethodPost,
				Path:   "/v1/schedules",
			},
			want: httpResp{
				Status: 403,
				Body: `{
					"code": "unauthorized", 
					"cause": "authorization header must be specified", 
					"message": "Client is not authorized for the requested action"
				}`,
			},
		},
		{
			title: "InvalidRequest",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodPatch,
				Path:    "/v1/schedules/s1",
				Body:    `{"invalid": "json}`,
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 400,
				Body: `{
					"code": "bad_request",
					"cause": "invalid json body: unexpected EOF",
					"message": "Request is not valid"
				}`,
			},
		},
		{
			title: "NotFound",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					Update(mock.Anything, "s1", schedule.Updates{Crontab: "@every 2h"}).
					Return(nil, clockwork.ErrNotFound).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodPatch,
				Path:    "/v1/schedules/s1",
				Body:    `{"crontab": "@every 2h"}`,
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 404,
				Body: `{
					"code": "not_found",
					"message": "Requested resource not found"
				}`,
			},
		},
		{
			title: "Success",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}
				scheduleSvc.EXPECT().
					Update(mock.Anything, "s1", schedule.Updates{Crontab: "@every 1h"}).
					Return(&sampleSchedule, nil).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodPatch,
				Path:    "/v1/schedules/s1",
				Body:    `{"crontab": "@every 1h"}`,
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 200,
				Body: jsonBody(map[string]any{
					"id":               "s1",
					"tags":             []string{"team:foo"},
					"status":           "ACTIVE",
					"crontab":          "@every 1h",
					"version":          1,
					"payload":          "{}",
					"category":         "foo",
					"client_id":        "foo",
					"created_at":       "2022-02-22T06:03:22Z",
					"updated_at":       "2022-02-22T06:03:22Z",
					"enqueue_count":    1,
					"last_enqueued_at": "2022-02-22T06:03:22Z",
				}),
			},
		},
	})
}

func Test_deleteSchedule(t *testing.T) {
	runAll(t, []endpointTestCase{
		{
			title: "AuthorizationError",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				scheduleSvc := &mocks.ScheduleService{}

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method: http.MethodDelete,
				Path:   "/v1/schedules/foo",
			},
			want: httpResp{
				Status: 403,
				Body: `{
					"code": "unauthorized",
					"cause": "authorization header must be specified",
					"message": "Client is not authorized for the requested action"
				}`,
			},
		},
		{
			title: "InternalIssue",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}

				scheduleSvc.EXPECT().
					Delete(mock.Anything, "foo").
					Return(clockwork.ErrInternal).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodDelete,
				Path:    "/v1/schedules/foo",
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 500,
				Body: `{
					"code": "internal_error",
					"message": "Some unexpected error occurred"
				}`,
			},
		},
		{
			title: "NotFound",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}

				scheduleSvc.EXPECT().
					Delete(mock.Anything, "foo").
					Return(clockwork.ErrNotFound).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodDelete,
				Path:    "/v1/schedules/foo",
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 204,
			},
		},
		{
			title: "Success",
			setup: func(t *testing.T) (http.Handler, func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				scheduleSvc := &mocks.ScheduleService{}

				scheduleSvc.EXPECT().
					Delete(mock.Anything, "foo").
					Return(nil).
					Once()

				return server.Router("v0.1.0", nil, scheduleSvc, clientSvc, true), func() {
					clientSvc.AssertExpectations(t)
					scheduleSvc.AssertExpectations(t)
				}
			},
			request: httpReq{
				Method:  http.MethodDelete,
				Path:    "/v1/schedules/foo",
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: 204,
			},
		},
	})
}
