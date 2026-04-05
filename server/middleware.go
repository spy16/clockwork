package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
)

const headerRequestID = "X-Request-Id"

func panicRecovery() mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					err, ok := v.(error)
					if !ok {
						err = fmt.Errorf("%v", v)
					}
					log.Error().Err(err).Msgf("recovered from a panic")
					writeErrJSON(req, wr, clockwork.ErrInternal)
				}
			}()

			h.ServeHTTP(wr, req)
		})
	}
}

func clientAuth(clients ClientService, enableVerify bool) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			clientID, secret, ok := req.BasicAuth()
			if !ok {
				writeErrJSON(req, wr,
					clockwork.ErrUnauthorized.WithCausef("authorization header must be specified"))
				return
			}

			curClient, err := clients.GetClient(req.Context(), clientID)
			if err != nil && !errors.Is(err, clockwork.ErrNotFound) {
				writeErrJSON(req, wr,
					clockwork.ErrInternal.WithCausef("%s", err.Error()))
				return
			}

			if curClient == nil || (enableVerify && !curClient.Verify(secret)) {
				writeErrJSON(req, wr,
					clockwork.ErrUnauthorized.WithCausef("client id or secret is not valid"))
				return
			}

			req = req.WithContext(client.Context(req.Context(), *curClient))
			next.ServeHTTP(wr, req)
		})
	}
}

func requestID() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			rid := strings.TrimSpace(req.Header.Get(headerRequestID))
			if rid == "" {
				rid = xid.New().String()
			}

			headers := req.Header.Clone()
			headers.Set(headerRequestID, rid)

			wr.Header().Set(headerRequestID, rid)
			req.Header = headers
			next.ServeHTTP(wr, req)
		})
	}
}

func ctxLogger() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			t := time.Now()
			span := trace.FromContext(req.Context())

			clientID, _, _ := req.BasicAuth()
			reqLogger := log.With().Fields(map[string]any{
				"path":       req.URL.Path,
				"method":     req.Method,
				"request_id": req.Header.Get(headerRequestID),
				"client_id":  clientID,
				"trace_id":   span.SpanContext().TraceID.String(),
			}).Logger()

			wrapped := &wrappedWriter{ResponseWriter: wr}
			next.ServeHTTP(wrapped, req.WithContext(reqLogger.WithContext(req.Context())))

			entry := reqLogger.Info()
			if wrapped.err != nil {
				entry = reqLogger.Warn()
			}

			entry.
				Err(wrapped.err).
				Str("response_time", time.Since(t).String()).
				Int("status_code", wrapped.status).
				Msg("request handled")
		})
	}
}

func openCensus() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		oc := &ochttp.Handler{
			Handler:          next,
			FormatSpanName:   formatSpanName,
			IsPublicEndpoint: false,
		}
		return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			route := mux.CurrentRoute(req)

			pathTpl := req.URL.Path
			if route != nil {
				pathTpl, _ = route.GetPathTemplate()
			}

			ctx, _ := tag.New(req.Context(),
				tag.Insert(ochttp.KeyServerRoute, pathTpl),
				tag.Insert(ochttp.Method, req.Method),
			)
			oc.ServeHTTP(wr, req.WithContext(ctx))
		})
	}
}

func formatSpanName(req *http.Request) string {
	route := mux.CurrentRoute(req)

	pathTpl := req.URL.Path
	if route != nil {
		pathTpl, _ = route.GetPathTemplate()
	}

	return fmt.Sprintf("%s %s", req.Method, pathTpl)
}

type wrappedWriter struct {
	http.ResponseWriter
	status int
	err    error
}

func (wr *wrappedWriter) WriteHeader(statusCode int) {
	wr.status = statusCode
	wr.ResponseWriter.WriteHeader(statusCode)
}
