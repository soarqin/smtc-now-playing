//go:build smtc_test

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"smtc-now-playing/internal/smtc"
)

func main() {
	s := smtc.New(smtc.Options{
		OnInfo: func(data smtc.InfoData) {
			fmt.Println(data.Artist, data.Title, data.ThumbnailContentType)
		},
		OnProgress: func(data smtc.ProgressData) {
			fmt.Println(data.Position, data.Duration, data.Status)
		},
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := s.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "failed to start: %v\n", err)
		os.Exit(1)
	}
}
