package httputil

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

const gracePeriod = 5 * time.Second

// Serve starts the http server that invokes the handler for incoming
// requests. When the context is cancelled, it performs graceful shutdown of
// the http server with a grace period of 5 seconds. If server does not stop
// within 5 sec, log.Fatalf is called which stops the host process.
func Serve(ctx context.Context, addr string, h http.Handler) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: h,
	}

	go func() {
		<-ctx.Done()

		ctxShutDown, cancel := context.WithTimeout(context.Background(), gracePeriod)
		defer cancel()

		if err := srv.Shutdown(ctxShutDown); err != nil {
			log.Fatal().Msg("graceful shutdown timed out: exiting process.")
		}
	}()

	if err := srv.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
	return nil
}
