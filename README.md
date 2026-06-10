# Twitch URL Extractor

A Golang package porting the Streamlink's Twitch URL extraction flow.

It mirrors the important steps:

1. Match a Twitch live, VOD, clip, or `player.twitch.tv` URL.
2. Request a Twitch GraphQL playback access token.
3. Build the authenticated `usher.ttvnw.net` HLS playlist URL.
4. Fetch and parse the HLS multivariant playlist into quality-named stream URLs.
5. For clips, use the GraphQL `sourceURL` values directly and append the clip token/signature.

Installation:

```bash 
go get github.com/c2bw/twitch-url-extractor
```

Usage:

```go
package main

import (
	"context"
	"fmt"
	"log"

	twitch "github.com/c2bw/twitch-url-extractor"
)

func main() {
	client := twitch.NewClient(nil)

	streams, err := client.Streams(context.Background(), "https://www.twitch.tv/example_channel")
	if err != nil {
		log.Fatal(err)
	}

	stream, ok := twitch.BestStream(streams)
	if !ok {
		log.Fatal("no streams found")
	}

	fmt.Println(stream.URL)
}
```

`Streams` accepts Twitch live channel, VOD, clip, and `player.twitch.tv` URLs. It returns a map keyed by quality name, such as `source`, `1080p60`, `720p`, or `audio_only`. Use `BestStream` to pick the highest-quality playable URL, or select a specific quality from the map:

```go
source := streams["source"].URL
```
