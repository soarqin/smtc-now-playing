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
	thumbnailContentType := ""
	thumbnailData := []byte{}
	position := 0
	duration := 0
	status := 0
	for {
		time.Sleep(200 * time.Millisecond)
		smtc.Update()
		dirty := smtc.RetrieveDirtyData(&artist, &title, &thumbnailContentType, &thumbnailData, &position, &duration, &status)
		if dirty&1 != 0 {
			fmt.Println(artist, title, thumbnailContentType)
		}
		if dirty&2 != 0 {
			fmt.Println(position, duration, status)
		}
	}
}
