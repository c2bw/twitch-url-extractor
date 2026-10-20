// Package twitch extracts playable stream URLs from Twitch live, VOD, clip,
// and player URLs.
package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	twitchClientID   = "kimne78kx3ncx6brgo4mv6wki5h1ko"
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:157.0) Gecko/20100101 Firefox/157.0"
)

type Client struct {
	http *http.Client
	// SupportedCodecs signals codec preferences to Usher (e.g. []string{"h264", "h265", "av1"}).
	// Defaults to []string{"h264"} when empty.
	SupportedCodecs []string
}

type Stream struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

type target struct {
	channel string
	videoID string
	clipID  string
	offset  time.Duration
}

type playbackToken struct {
	Signature string
	Value     string
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{http: httpClient}
}

func (c *Client) Streams(ctx context.Context, rawURL string) (map[string]Stream, error) {
	target, err := parseTwitchURL(rawURL)
	if err != nil {
		return nil, err
	}

	switch {
	case target.videoID != "":
		return c.videoStreams(ctx, target.videoID, target.offset)
	case target.clipID != "":
		return c.clipStreams(ctx, target.clipID)
	case target.channel != "":
		return c.liveStreams(ctx, target.channel)
	default:
		return nil, errors.New("unsupported Twitch URL")
	}
}

func parseTwitchURL(rawURL string) (target, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return target{}, err
	}

	var t target
	if strings.EqualFold(u.Hostname(), "player.twitch.tv") {
		t.channel = u.Query().Get("channel")
		t.videoID = videoID(u.Query().Get("video"))
	} else {
		t.videoID = videoIDFromPath(u)
		t.clipID = clipIDFromPath(u)
	}
	if t.videoID == "" && t.clipID == "" && t.channel == "" && isTwitchHost(u.Hostname()) {
		t.channel = channelFromPath(u.Path)
	}
	t.offset = parseOffset(u.Query().Get("t"))

	if t.channel == "" && t.videoID == "" && t.clipID == "" {
		return target{}, errors.New("URL did not match Twitch live, VOD, clip, or player patterns")
	}
	return t, nil
}

func videoIDFromPath(u *url.URL) string {
	if !isTwitchHost(u.Hostname()) {
		return ""
	}

	parts := pathSegments(u.Path)
	if len(parts) == 2 && isVideoPath(parts[0]) {
		return videoID(parts[1])
	}
	if len(parts) == 3 && isVideoPath(parts[1]) {
		return videoID(parts[2])
	}
	return ""
}

func clipIDFromPath(u *url.URL) string {
	parts := pathSegments(u.Path)
	switch {
	case strings.EqualFold(u.Hostname(), "clips.twitch.tv") && len(parts) == 1:
		return parts[0]
	case isTwitchHost(u.Hostname()) && len(parts) == 2 && strings.EqualFold(parts[0], "clip"):
		return parts[1]
	case isTwitchHost(u.Hostname()) && len(parts) == 3 && strings.EqualFold(parts[1], "clip"):
		return parts[2]
	default:
		return ""
	}
}

func videoID(value string) string {
	value = strings.TrimPrefix(value, "v")
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return value
}

func isVideoPath(value string) bool {
	switch strings.ToLower(value) {
	case "v", "video", "videos":
		return true
	default:
		return false
	}
}

func pathSegments(path string) []string {
	if path = strings.Trim(path, "/"); path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func isTwitchHost(host string) bool {
	host = strings.ToLower(host)
	return host == "twitch.tv" || strings.HasSuffix(host, ".twitch.tv") && host != "clips.twitch.tv"
}

func channelFromPath(path string) string {
	parts := pathSegments(path)
	if len(parts) != 1 || parts[0] == "" {
		return ""
	}

	channel := parts[0]
	lower := strings.ToLower(channel)
	if lower == "clip" || lower == "videos" || lower == "v" || lower == "video" {
		return ""
	}
	return channel
}

func (c *Client) liveStreams(ctx context.Context, channel string) (map[string]Stream, error) {
	token, err := c.accessToken(ctx, true, channel)
	if err != nil {
		return nil, err
	}

	playlistURL := c.usherURL(
		fmt.Sprintf("/api/v2/channel/hls/%s.m3u8", strings.ToLower(channel)),
		map[string]string{
			"sig":        token.Signature,
			"token":      token.Value,
			"fast_bread": "true",
		},
	)

	headers := map[string]string{
		"Referer": "https://player.twitch.tv",
		"Origin":  "https://player.twitch.tv",
	}
	return c.hlsStreams(ctx, playlistURL, 0, headers)
}

func (c *Client) videoStreams(ctx context.Context, videoID string, offset time.Duration) (map[string]Stream, error) {
	token, err := c.accessToken(ctx, false, videoID)
	if err != nil {
		return nil, err
	}

	playlistURL := c.usherURL(
		fmt.Sprintf("/vod/v2/%s.m3u8", videoID),
		map[string]string{
			"nauthsig": token.Signature,
			"nauth":    token.Value,
		},
	)

	return c.hlsStreams(ctx, playlistURL, offset, nil)
}

func (c *Client) hlsStreams(ctx context.Context, playlistURL string, offset time.Duration, headers map[string]string) (map[string]Stream, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, playlistURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("usher playlist request failed: %s: %s", res.Status, string(body))
	}

	return parseVariantPlaylist(playlistURL, string(body), offset), nil
}

func (c *Client) usherURL(endpoint string, extra map[string]string) string {
	codecs := "h264"
	if len(c.SupportedCodecs) > 0 {
		codecs = strings.Join(c.SupportedCodecs, ",")
	}
	return usherURLWithCodecs(endpoint, codecs, extra)
}

func usherURL(endpoint string, extra map[string]string) string {
	return usherURLWithCodecs(endpoint, "h264", extra)
}

func usherURLWithCodecs(endpoint, codecs string, extra map[string]string) string {
	values := url.Values{
		"platform":                   {"web"},
		"p":                          {fmt.Sprint(rand.Intn(999999))},
		"allow_source":               {"true"},
		"allow_audio_only":           {"true"},
		"playlist_include_framerate": {"true"},
		"multigroup_video":           {"true"},
		"supported_codecs":           {codecs},
	}
	for key, value := range extra {
		values.Set(key, value)
	}
	return (&url.URL{
		Scheme:   "https",
		Host:     "usher.ttvnw.net",
		Path:     endpoint,
		RawQuery: values.Encode(),
	}).String()
}

func (c *Client) accessToken(ctx context.Context, isLive bool, channelOrVod string) (playbackToken, error) {
	variables := map[string]any{
		"isLive":     isLive,
		"login":      "",
		"isVod":      !isLive,
		"vodID":      "",
		"playerType": "embed",
		"platform":   "site",
	}
	if isLive {
		variables["login"] = channelOrVod
	} else {
		variables["vodID"] = channelOrVod
	}

	payload := gqlPersistedQuery(
		"PlaybackAccessToken",
		"ed230aa1e33e07eebb8928504583da78a5173989fadfb1ac94be06a04f3cdbe9",
		variables,
	)

	var response struct {
		Data struct {
			Stream *struct {
				Value     string `json:"value"`
				Signature string `json:"signature"`
			} `json:"streamPlaybackAccessToken"`
			Video *struct {
				Value     string `json:"value"`
				Signature string `json:"signature"`
			} `json:"videoPlaybackAccessToken"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}

	if err := c.postGQL(ctx, payload, &response); err != nil {
		return playbackToken{}, err
	}
	if len(response.Errors) > 0 {
		return playbackToken{}, errors.New(response.Errors[0].Message)
	}
	if response.Error != "" || response.Message != "" {
		if response.Error != "" && response.Message != "" {
			return playbackToken{}, fmt.Errorf("%s: %s", response.Error, response.Message)
		}
		return playbackToken{}, errors.New(response.Error + response.Message)
	}

	var sig, value string
	if isLive && response.Data.Stream != nil {
		sig, value = response.Data.Stream.Signature, response.Data.Stream.Value
	} else if !isLive && response.Data.Video != nil {
		sig, value = response.Data.Video.Signature, response.Data.Video.Value
	}
	if sig == "" || value == "" {
		return playbackToken{}, errors.New("no playback token returned; stream may be offline or unavailable")
	}

	return playbackToken{
		Signature: sig,
		Value:     value,
	}, nil
}

func (c *Client) postGQL(ctx context.Context, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://gql.twitch.tv/gql", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Client-ID", twitchClientID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent)

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("gql request failed: %s: %s", res.Status, string(data))
	}
	return json.Unmarshal(data, out)
}

func gqlPersistedQuery(operationName, sha256Hash string, variables map[string]any) map[string]any {
	return map[string]any{
		"operationName": operationName,
		"extensions": map[string]any{
			"persistedQuery": map[string]any{
				"version":    1,
				"sha256Hash": sha256Hash,
			},
		},
		"variables": variables,
	}
}
