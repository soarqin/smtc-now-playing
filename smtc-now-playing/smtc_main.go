//go:build smtc_test

package main

import (
	"fmt"
	"time"
)

func main() {
	smtc := SmtcCreate()
	smtc.Init()
	artist := ""
	title := ""
	thumbnailPath := ""
	position := 0
	duration := 0
	status := 0
	for {
		time.Sleep(200 * time.Millisecond)
		smtc.Update()
		dirty := smtc.RetrieveDirtyData(&artist, &title, &thumbnailPath, &position, &duration, &status)
		if dirty&1 != 0 {
			fmt.Println(artist, title, thumbnailPath)
		}
		if dirty&2 != 0 {
			fmt.Println(position, duration, status)
		}
	}
}
