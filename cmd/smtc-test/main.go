//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"smtc-now-playing/internal/smtc"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	svc := smtc.New(smtc.Options{})
	events := svc.Subscribe(32)
	defer svc.Unsubscribe(events)

	go func() { _ = svc.Run(ctx) }()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			switch e := ev.(type) {
			case smtc.InfoEvent:
				fmt.Printf("INFO: artist=%q title=%q\n", e.Data.Artist, e.Data.Title)
			case smtc.ProgressEvent:
				fmt.Printf("PROGRESS: pos=%d dur=%d status=%d\n", e.Data.Position, e.Data.Duration, e.Data.Status)
			case smtc.SessionsChangedEvent:
				fmt.Printf("SESSIONS: %v\n", e.Sessions)
			case smtc.DeviceChangedEvent:
				fmt.Printf("DEVICE: %s\n", e.AppID)
			}
		}
	}
}
