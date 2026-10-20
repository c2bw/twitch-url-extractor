package twitch

import (
	"testing"
	"time"
)

func TestParseTwitchURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		want    target
		wantErr bool
	}{
		{
			name:   "live channel",
			rawURL: "https://www.twitch.tv/example_channel",
			want:   target{channel: "example_channel"},
		},
		{
			name:   "vod with offset",
			rawURL: "https://www.twitch.tv/videos/123456?t=1h2m3s",
			want:   target{videoID: "123456", offset: time.Hour + 2*time.Minute + 3*time.Second},
		},
		{
			name:   "old channel vod path",
			rawURL: "https://www.twitch.tv/example/videos/123456",
			want:   target{videoID: "123456"},
		},
		{
			name:   "clip host",
			rawURL: "https://clips.twitch.tv/FancyClipSlug",
			want:   target{clipID: "FancyClipSlug"},
		},
		{
			name:   "channel clip path",
			rawURL: "https://www.twitch.tv/example/clip/FancyClipSlug",
			want:   target{clipID: "FancyClipSlug"},
		},
		{
			name:   "player video",
			rawURL: "https://player.twitch.tv/?video=v123456&parent=example.com",
			want:   target{videoID: "123456"},
		},
		{
			name:    "reject partial vod id",
			rawURL:  "https://www.twitch.tv/videos/123abc",
			wantErr: true,
		},
		{
			name:    "reject clip with extra path",
			rawURL:  "https://clips.twitch.tv/FancyClipSlug/extra",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTwitchURL(tt.rawURL)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTwitchURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseTwitchURL() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestUsherURL(t *testing.T) {
	client := NewClient(nil)
	client.SupportedCodecs = []string{"h264", "h265", "av1"}

	rawURL := client.usherURL("/api/v2/channel/hls/example.m3u8", map[string]string{
		"sig":        "sig123",
		"token":      "tok123",
		"fast_bread": "true",
	})
	q := mustQuery(t, rawURL)
	if got := q.Get("multigroup_video"); got != "true" {
		t.Fatalf("multigroup_video = %q, want true", got)
	}
	if got := q.Get("supported_codecs"); got != "h264,h265,av1" {
		t.Fatalf("supported_codecs = %q, want h264,h265,av1", got)
	}
}
