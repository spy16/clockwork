package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/server"
	"github.com/spy16/clockwork/server/mocks"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("DISABLE_PANIC_RECOVERY", "true") // disable panic recovery middleware for tests.
	os.Exit(m.Run())
}

func TestRouter(t *testing.T) {
	runAll(t, []endpointTestCase{
		{
			title: "Ping",
			request: httpReq{
				Method: http.MethodGet,
				Path:   "/ping",
			},
			want: httpResp{
				Status: http.StatusOK,
				Body:   `{"status": "ok"}`,
			},
		},
		{
			title: "NotFound",
			request: httpReq{
				Method: http.MethodGet,
				Path:   "/non-existent-route",
			},
			want: httpResp{
				Status: http.StatusNotFound,
				Body:   `{"cause":"endpoint 'GET /non-existent-route' is non-existent", "code":"not_found", "message":"Path not found"}`,
			},
		},
		{
			title: "MethodNotAllowed",
			request: httpReq{
				Method: http.MethodDelete,
				Path:   "/ping",
			},
			want: httpResp{
				Status: http.StatusMethodNotAllowed,
				Body:   `{"cause":"method DELETE is not allowed for /ping", "code":"method_not_allowed", "message":"Method not allowed"}`,
			},
		},
	})
}

func Test_systemInfo(t *testing.T) {
	runAll(t, []endpointTestCase{
		{
			title: "Success",
			setup: func(t *testing.T) (router http.Handler, done func()) {
				clientSvc := &mocks.ClientService{}
				clientSvc.EXPECT().
					GetClient(mock.Anything, "foo").
					Return(&client.Client{
						ID: "foo",
						// BCrypt hash of secret 'bar'
						Secret: "$2a$04$4vhyCEoZx5Uog5EsfQpBvulYB3YkyQeIbHWiJT54QFjo6/59WsHZC",
					}, nil).
					Once()

				return server.Router("1.0.0", nil, nil, clientSvc, true), nil
			},
			request: httpReq{
				Method:  http.MethodGet,
				Path:    "/system",
				Headers: defaultHeaders,
			},
			want: httpResp{
				Status: http.StatusOK,
				Body: jsonBody(map[string]any{
					"clockwork_version": "1.0.0",
					"go_runtime":        runtime.Version(),
				}),
			},
		},
	})
}

func runAll(t *testing.T, table []endpointTestCase) {
	t.Parallel()

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			req := httptest.NewRequest(tt.request.Method, "http://localhost"+tt.request.Path, strings.NewReader(tt.request.Body))
			req.Header = tt.request.Headers
			if req.Header == nil {
				req.Header = map[string][]string{}
			}
			wr := httptest.NewRecorder()

			var srv http.Handler
			if tt.setup == nil {
				srv = server.Router("1.0.0", nil, nil, nil, true)
			} else {
				router, done := tt.setup(t)
				if done != nil {
					defer done()
				}
				srv = router
			}
			srv.ServeHTTP(wr, req)

			assertResponse(t, tt.want, wr.Result())
		})
	}
}

func assertResponse(t *testing.T, want httpResp, got *http.Response) {
	defer func() { _ = got.Body.Close() }()

	assert.Equal(t, want.Status, got.StatusCode)
	if len(want.Headers) > 0 {
		assert.Equal(t, got.Header, want.Headers)
	}

	var bodyStr string
	if got.Body != nil {
		responseBody, err := io.ReadAll(got.Body)
		require.NoError(t, err)
		bodyStr = strings.TrimSpace(string(responseBody))
	}

	if want.Body != "" || bodyStr != "" {
		assert.JSONEq(t, want.Body, bodyStr)
	}
}

type httpResp struct {
	Status  int
	Body    string
	Headers http.Header
}

type httpReq struct {
	Method  string
	Path    string
	Headers http.Header
	Body    string
}

type endpointTestCase struct {
	title   string
	setup   func(t *testing.T) (router http.Handler, done func())
	request httpReq
	want    httpResp
}

func jsonBody(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
