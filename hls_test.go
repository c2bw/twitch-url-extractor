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

func TestParseVariantPlaylistUsherV2(t *testing.T) {
	const playlist = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=9224531,CODECS="hev1.1.6.L150.90.0.0.0.0.0,mp4a.40.2",RESOLUTION=2560x1440,FRAME-RATE=60.000,STABLE-VARIANT-ID="1440p60",IVS-NAME="1440p60",IVS-GROUPS="landscape",IVS-VARIANT-SOURCE="source"
/1440p60.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=5224532,CODECS="hev1.1.6.L123.90.0.0.0.0.0,mp4a.40.2",RESOLUTION=1080x1920,FRAME-RATE=60.000,STABLE-VARIANT-ID="1080p60-portrait",IVS-NAME="1080p60",IVS-GROUPS="portrait",IVS-VARIANT-SOURCE="source"
/1080p60-portrait.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=7635509,CODECS="avc1.4D402A,mp4a.40.2",RESOLUTION=1920x1080,FRAME-RATE=60.000,STABLE-VARIANT-ID="1080p60",IVS-NAME="1080p60",IVS-GROUPS="landscape",IVS-VARIANT-SOURCE="source"
/1080p60.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=3233003,CODECS="avc1.4D4020,mp4a.40.2",RESOLUTION=1280x720,FRAME-RATE=60.000,STABLE-VARIANT-ID="720p60",IVS-NAME="720p60",IVS-GROUPS="landscape",IVS-VARIANT-SOURCE="source"
/720p60.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1340210,CODECS="avc1.4D401F,mp4a.40.2",RESOLUTION=852x480,FRAME-RATE=30.000,STABLE-VARIANT-ID="480p30",IVS-NAME="480p",IVS-GROUPS="landscape",IVS-VARIANT-SOURCE="source"
/480p30.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=645708,CODECS="avc1.4D401E,mp4a.40.2",RESOLUTION=640x360,FRAME-RATE=30.000,STABLE-VARIANT-ID="360p30",IVS-NAME="360p",IVS-GROUPS="landscape",IVS-VARIANT-SOURCE="source"
/360p30.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=248074,CODECS="avc1.4D400C,mp4a.40.2",RESOLUTION=284x160,FRAME-RATE=30.000,STABLE-VARIANT-ID="160p30",IVS-NAME="160p",IVS-GROUPS="landscape",IVS-VARIANT-SOURCE="source"
/160p30.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=3250495,CODECS="avc1.4D4020,mp4a.40.2",RESOLUTION=720x1280,FRAME-RATE=60.000,STABLE-VARIANT-ID="720p60-portrait",IVS-NAME="720p60",IVS-GROUPS="portrait",IVS-VARIANT-SOURCE="source"
/720p60-portrait.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=1347463,CODECS="avc1.4D401F,mp4a.40.2",RESOLUTION=480x852,FRAME-RATE=30.000,STABLE-VARIANT-ID="480p30-portrait",IVS-NAME="480p",IVS-GROUPS="portrait",IVS-VARIANT-SOURCE="source"
/480p30-portrait.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=649640,CODECS="avc1.4D401E,mp4a.40.2",RESOLUTION=360x640,FRAME-RATE=30.000,STABLE-VARIANT-ID="360p30-portrait",IVS-NAME="360p",IVS-GROUPS="portrait",IVS-VARIANT-SOURCE="source"
/360p30-portrait.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=249454,CODECS="avc1.4D400C,mp4a.40.2",RESOLUTION=160x284,FRAME-RATE=30.000,STABLE-VARIANT-ID="160p30-portrait",IVS-NAME="160p",IVS-GROUPS="portrait",IVS-VARIANT-SOURCE="source"
/160p30-portrait.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=210452,CODECS="mp4a.40.2",STABLE-VARIANT-ID="audio_only",IVS-NAME="Audio Only",IVS-GROUPS="landscape,portrait",IVS-VARIANT-SOURCE="source"
/audio_only.m3u8
`

	streams := parseVariantPlaylist("https://usher.ttvnw.net/api/v2/channel/hls/example.m3u8", playlist, 0)

	wantKeys := []string{
		"1440p60",
		"1080p60",
		"720p60",
		"480p30",
		"360p30",
		"160p30",
		"1920p60_portrait",
		"1280p60_portrait",
		"852p30_portrait",
		"640p30_portrait",
		"284p30_portrait",
		"audio_only",
	}
	if len(streams) != len(wantKeys) {
		t.Fatalf("len(streams) = %d, want %d: %+v", len(streams), len(wantKeys), streams)
	}
	for _, k := range wantKeys {
		if _, ok := streams[k]; !ok {
			t.Fatalf("missing stream key %q in %+v", k, streams)
		}
	}

	best, ok := BestStream(streams)
	if !ok {
		t.Fatal("BestStream() ok = false")
	}
	if best.Name != "1440p60" {
		t.Fatalf("BestStream() = %+v, want 1440p60", best)
	}
}

func TestBestStream(t *testing.T) {
	streams := map[string]Stream{
		"audio_only":       {Name: "audio_only", URL: "https://example.com/audio.m3u8"},
		"720p60":           {Name: "720p60", URL: "https://example.com/720.m3u8"},
		"source":           {Name: "source"},
		"1920p60_portrait": {Name: "1920p60_portrait", URL: "https://example.com/portrait.m3u8"},
		"1080p60_alt1":     {Name: "1080p60_alt1", URL: "https://example.com/1080-alt.m3u8"},
		"1080p60":          {Name: "1080p60", URL: "https://example.com/1080.m3u8"},
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
