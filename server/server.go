package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"runtime"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/schedule"
)

//go:embed swagger/**
var swaggerFS embed.FS

//go:generate mockery --with-expecter --keeptree --case snake --name ScheduleService
//go:generate mockery --with-expecter --keeptree --case snake --name ClientService

type ScheduleService interface {
	List(ctx context.Context, offset, count int) ([]schedule.Schedule, error)
	Fetch(ctx context.Context, scheduleID string) (*schedule.Schedule, error)
	Create(ctx context.Context, sc schedule.Schedule) (*schedule.Schedule, error)
	Update(ctx context.Context, id string, update schedule.Updates) (*schedule.Schedule, error)
	Delete(ctx context.Context, scheduleID string) error
}

type ClientService interface {
	GetClient(ctx context.Context, id string) (*client.Client, error)
	DeleteClient(ctx context.Context, id string) error
	RegisterClient(ctx context.Context, cl client.Client) (*client.Client, error)
}

// Router returns HTTP router for clockwork service REST APIs.
func Router(clockworkVersion string, scheduler schedule.Scheduler, scheduleSvc ScheduleService,
	clientSvc ClientService, verifyClient bool) http.Handler {
	router := mux.NewRouter()
	if os.Getenv("DISABLE_PANIC_RECOVERY") != "true" {
		router.Use(panicRecovery())
	}
	router.Use(
		openCensus(),
		requestID(),
		ctxLogger(),
	)
	router.NotFoundHandler = notFoundHandler()
	router.MethodNotAllowedHandler = methodNotAllowedHandler()

	withAuth := clientAuth(clientSvc, verifyClient)

	// system APIs.
	router.Handle("/ping", pingHandler()).Methods(http.MethodGet)
	router.Handle("/system", withAuth(systemInfoHandler(clockworkVersion, scheduler)))

	// swagger document
	swaggerDist, err := fs.Sub(swaggerFS, "swagger")
	if err != nil {
		panic(err)
	}
	router.PathPrefix("/swagger/").Handler(http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerDist))))

	// schedule management APIs - v1.
	scheduleAPI := router.PathPrefix("/v1/schedules").Subrouter()
	scheduleAPI.Use(withAuth)
	scheduleAPI.Handle("", listSchedules(scheduleSvc)).Methods(http.MethodGet)
	scheduleAPI.Handle("", createSchedule(scheduleSvc)).Methods(http.MethodPost)
	scheduleAPI.Handle("/{scheduleID}", getSchedule(scheduleSvc)).Methods(http.MethodGet)
	scheduleAPI.Handle("/{scheduleID}", updateSchedule(scheduleSvc)).Methods(http.MethodPatch)
	scheduleAPI.Handle("/{scheduleID}", deleteSchedule(scheduleSvc)).Methods(http.MethodDelete)

	// client management APIs - v1.
	clientAPI := router.PathPrefix("/v1/clients").Subrouter()
	clientAPI.Handle("", createClient(clientSvc)).Methods(http.MethodPost)
	clientAPI.Handle("/{clientID}", getClient(clientSvc)).Methods(http.MethodGet)
	clientAPI.Handle("/{clientID}", withAuth(deleteClient(clientSvc))).Methods(http.MethodDelete)

	cors := handlers.CORS(handlers.AllowedOrigins([]string{"*"}))
	return cors(router)
}

func systemInfoHandler(clockworkVersion string, scheduler schedule.Scheduler) http.Handler {
	schedulerWithStats, supported := scheduler.(interface {
		Stats(ctx context.Context) (map[string]any, error)
	})

	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		systemInfo := map[string]any{
			"clockwork_version": clockworkVersion,
			"go_runtime":        runtime.Version(),
		}

		if supported {
			schedulerStats, err := schedulerWithStats.Stats(req.Context())
			if err != nil {
				writeJSON(req, wr, http.StatusInternalServerError, err)
				return
			}
			systemInfo["scheduler"] = schedulerStats
		}

		writeJSON(req, wr, http.StatusOK, systemInfo)
	})
}

func notFoundHandler() http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		writeJSON(req, wr, http.StatusNotFound, clockwork.Error{
			Code:    "not_found",
			Message: "Path not found",
			Cause:   fmt.Sprintf("endpoint '%s %s' is non-existent", req.Method, req.URL.Path),
		})
	})
}

func methodNotAllowedHandler() http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		writeJSON(req, wr, http.StatusMethodNotAllowed, clockwork.Error{
			Code:    "method_not_allowed",
			Message: "Method not allowed",
			Cause:   fmt.Sprintf("method %s is not allowed for %s", req.Method, req.URL.Path),
		})
	})
}

func pingHandler() http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		writeJSON(req, wr, http.StatusOK, map[string]any{
			"status": "ok",
		})
	})
}

// writeJSON encodes 'v' as JSON and writes response body with appropriate headers.
func writeJSON(_ *http.Request, wr http.ResponseWriter, status int, v any) {
	if status == http.StatusNoContent {
		wr.WriteHeader(status)
		return
	}
	wr.Header().Set("Content-type", "application/json")
	wr.WriteHeader(status)

	enc := json.NewEncoder(wr)

	switch val := v.(type) {
	case clockwork.Error:
		_ = enc.Encode(val)

	case error:
		_ = enc.Encode(clockwork.ErrInternal.WithCausef("%s", val.Error()))

	default:
		_ = enc.Encode(v)
	}
}

func writeErrJSON(req *http.Request, wr http.ResponseWriter, err error) {
	if wrappedWr, ok := wr.(*wrappedWriter); ok {
		wrappedWr.err = err
	}

	switch {
	case errors.Is(err, clockwork.ErrNotFound):
		writeJSON(req, wr, http.StatusNotFound, err)

	case errors.Is(err, clockwork.ErrInvalid):
		writeJSON(req, wr, http.StatusBadRequest, err)

	case errors.Is(err, clockwork.ErrConflict):
		writeJSON(req, wr, http.StatusConflict, err)

	case errors.Is(err, clockwork.ErrUnauthorized):
		writeJSON(req, wr, http.StatusForbidden, err)

	case errors.Is(err, clockwork.ErrUnsupported):
		writeJSON(req, wr, http.StatusUnprocessableEntity, err)

	default:
		writeJSON(req, wr, http.StatusInternalServerError, err)
	}
}
