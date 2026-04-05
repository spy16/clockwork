package main

import (
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func cmdTimeUtil() *cobra.Command {
	return &cobra.Command{
		Use:   "epoch [unix-timestamp]",
		Short: "🕒 Convert unix timestamps to local date-time string",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			t := time.Now()
			if len(args) == 1 {
				v, err := strconv.Atoi(args[0])
				if err != nil {
					log.Fatal().Msgf("invalid timestamp '%s': %v", args[0], err)
				}

				t = time.Unix(int64(v), 0)
			}

			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
				"time_utc":      t.UTC(),
				"time_local":    t,
				"seconds_epoch": t.Unix(),
			})
		},
	}
}
