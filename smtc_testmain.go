//go:build smtc_test

package main

import (
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

	if err := s.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start: %v\n", err)
		os.Exit(1)
	}

	// Block until Ctrl+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	s.Stop()
}
