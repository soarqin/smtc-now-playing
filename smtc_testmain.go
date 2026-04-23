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
	s := smtc.New(smtc.Options{})
	updates := s.Subscribe(16)
	defer s.Unsubscribe(updates)

	go func() {
		for ev := range updates {
			switch data := ev.(type) {
			case smtc.InfoEvent:
				fmt.Println(data.Data.Artist, data.Data.Title, data.Data.ThumbnailContentType)
			case smtc.ProgressEvent:
				fmt.Println(data.Data.Position, data.Data.Duration, data.Data.Status)
			case smtc.SessionsChangedEvent:
				fmt.Println("sessions", len(data.Sessions))
			case smtc.DeviceChangedEvent:
				fmt.Println("device", data.AppID)
			default:
				fmt.Println("unknown event", data)
			}
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := s.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "failed to start: %v\n", err)
		os.Exit(1)
	}
}
