package main

import (
	"log"

	"github.com/eka-care/eka-usage-sdk/sdks/go/ekausage"
)

func main() {
	client, err := ekausage.New(
		"scribe-api",
		ekausage.WithOnError(func(e error, ctx map[string]any) {
			log.Printf("sdk err: %v ctx=%v", e, ctx)
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Shutdown()

	client.Record("ws_123", "ekascribe", "transcription_minute", 8.2, "ok", nil)
	client.Log("ws_123", "error", "ffmpeg failed", "FFMPEG_EXIT_137", nil)
}
