package twitch

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseVariantPlaylist(t *testing.T) {
	const playlist = `#EXTM3U
#EXT-X-MEDIA:TYPE=VIDEO,GROUP-ID="chunked",NAME="chunked"
#EXT-X-MEDIA:TYPE=VIDEO,GROUP-ID="audio",NAME="audio"
#EXT-X-STREAM-INF:BANDWIDTH=6000000,RESOLUTION=1920x1080,FRAME-RATE=60.000,VIDEO="chunked"
chunked/index-dvr.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=3000000,RESOLUTION=1280x720,FRAME-RATE=60.000
https://cdn.example.com/720.m3u8?foo=bar
#EXT-X-STREAM-INF:BANDWIDTH=160000,VIDEO="audio"
audio/index.m3u8
`

	streams := parseVariantPlaylist("https://usher.ttvnw.net/vod/123.m3u8?nauth=x", playlist, 90*time.Second)

	source := streams["source"]
	if source.Name != "source" || source.Type != "hls" {
		t.Fatalf("source stream = %+v", source)
	}
	if got := mustQuery(t, source.URL).Get("start_offset"); got != "90" {
		t.Fatalf("source start_offset = %q, want 90", got)
	}
	if got := source.URL; !strings.HasPrefix(got, "https://usher.ttvnw.net/vod/chunked/") {
		t.Fatalf("source URL = %q", got)
	}

	stream720 := streams["720p60"]
	if stream720.URL == "" {
		t.Fatal("missing 720p60 stream")
	}
	query := mustQuery(t, stream720.URL)
	if got := query.Get("foo"); got != "bar" {
		t.Fatalf("foo query = %q, want bar", got)
	}
	if got := query.Get("start_offset"); got != "90" {
		t.Fatalf("720p60 start_offset = %q, want 90", got)
	}

	if _, ok := streams["audio_only"]; !ok {
		t.Fatal("missing audio_only stream")
	}
}

func TestBestStream(t *testing.T) {
	streams := map[string]Stream{
		"audio_only": {Name: "audio_only", URL: "https://example.com/audio.m3u8"},
		"720p60":     {Name: "720p60", URL: "https://example.com/720.m3u8"},
		"source":     {Name: "source"},
		"1080p60":    {Name: "1080p60", URL: "https://example.com/1080.m3u8"},
	}

	got, ok := BestStream(streams)
	if !ok {
		t.Fatal("BestStream() ok = false")
	}
	if got.Name != "1080p60" {
		t.Fatalf("BestStream() = %+v, want 1080p60", got)
	}
}

func mustQuery(t *testing.T, rawURL string) url.Values {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", rawURL, err)
	}
	return u.Query()
}
