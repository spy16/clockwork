package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
	"github.com/spy16/clockwork/schedule"
)

const paramScheduleID = "scheduleID"

func getSchedule(sched ScheduleService) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		id := mux.Vars(req)[paramScheduleID]

		sc, err := sched.Fetch(req.Context(), id)
		if err != nil {
			writeErrJSON(req, wr, err)
			return
		}

		var sr scheduleResource
		sr.from(*sc)
		writeJSON(req, wr, http.StatusOK, sr)
	})
}

func listSchedules(sched ScheduleService) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		// TODO: accept offset & count query params to paginate.
		schedules, err := sched.List(req.Context(), 0, -1)
		if err != nil {
			writeErrJSON(req, wr, err)
			return
		}

		if len(schedules) == 0 {
			writeJSON(req, wr, http.StatusOK, []schedule.Schedule{})
		} else {
			resp := make([]scheduleResource, len(schedules))
			for i, s := range schedules {
				resp[i].from(s)
			}
			writeJSON(req, wr, http.StatusOK, resp)
		}
	})
}

func createSchedule(sched ScheduleService) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		var sc scheduleResource
		if err := json.NewDecoder(req.Body).Decode(&sc); err != nil {
			writeErrJSON(req, wr, clockwork.ErrInvalid.WithCausef("invalid json body: %v", err))
			return
		}
		sc.ClientID = client.From(req.Context()).ID

		created, err := sched.Create(req.Context(), sc.to())
		if err != nil {
			writeErrJSON(req, wr, err)
			return
		}

		var resp scheduleResource
		resp.from(*created)
		writeJSON(req, wr, http.StatusCreated, resp)
	})
}

func updateSchedule(sched ScheduleService) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		var sc schedule.Updates
		if err := json.NewDecoder(req.Body).Decode(&sc); err != nil {
			writeErrJSON(req, wr, clockwork.ErrInvalid.WithCausef("invalid json body: %v", err))
			return
		}

		scheduleID := mux.Vars(req)[paramScheduleID]

		updated, err := sched.Update(req.Context(), scheduleID, sc)
		if err != nil {
			writeErrJSON(req, wr, err)
			return
		}

		writeJSON(req, wr, http.StatusOK, updated)
	})
}

func deleteSchedule(sched ScheduleService) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		id := mux.Vars(req)[paramScheduleID]

		err := sched.Delete(req.Context(), id)
		if err != nil && !errors.Is(err, clockwork.ErrNotFound) {
			writeErrJSON(req, wr, err)
			return
		}

		writeJSON(req, wr, http.StatusNoContent, nil)
	})
}

type scheduleResource struct {
	ID             string    `json:"id"`
	Tags           []string  `json:"tags"`
	Status         string    `json:"status"`
	Crontab        string    `json:"crontab"`
	Triggers       []int64   `json:"triggers,omitempty"`
	Version        int64     `json:"version"`
	Category       string    `json:"category"`
	Payload        string    `json:"payload"`
	ClientID       string    `json:"client_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ExecutionsDone int       `json:"enqueue_count"`
	LastExecutedAt time.Time `json:"last_enqueued_at"`
}

func (sr scheduleResource) to() schedule.Schedule {
	return schedule.Schedule{
		ID:              sr.ID,
		Tags:            sr.Tags,
		Status:          sr.Status,
		Crontab:         sr.Crontab,
		Triggers:        sr.Triggers,
		Version:         sr.Version,
		Category:        sr.Category,
		Payload:         sr.Payload,
		ClientID:        sr.ClientID,
		CreatedAt:       sr.CreatedAt.UTC(),
		UpdatedAt:       sr.UpdatedAt.UTC(),
		EnqueueCount:    sr.ExecutionsDone,
		NextExecutionAt: sr.LastExecutedAt.UTC(),
	}
}

func (sr *scheduleResource) from(sc schedule.Schedule) {
	*sr = scheduleResource{
		ID:             sc.ID,
		Tags:           sc.Tags,
		Status:         sc.Status,
		Crontab:        sc.Crontab,
		Triggers:       sc.Triggers,
		Version:        sc.Version,
		Category:       sc.Category,
		Payload:        sc.Payload,
		ClientID:       sc.ClientID,
		CreatedAt:      sc.CreatedAt.UTC(),
		UpdatedAt:      sc.UpdatedAt.UTC(),
		ExecutionsDone: sc.EnqueueCount,
		LastExecutedAt: sc.NextExecutionAt.UTC(),
	}
}
