package twitch

import (
	"context"
	"errors"
	"net/url"
	"strconv"
)

func (c *Client) clipStreams(ctx context.Context, clipID string) (map[string]Stream, error) {
	payload := gqlPersistedQuery(
		"VideoAccessToken_Clip",
		"993d9a5131f15a37bd16f32342c44ed1e0b1a9b968c6afdb662d2cddd595f6c5",
		map[string]any{
			"slug":     clipID,
			"platform": "web",
		},
	)

	var response struct {
		Data struct {
			Clip *struct {
				PlaybackAccessToken struct {
					Signature string `json:"signature"`
					Value     string `json:"value"`
				} `json:"playbackAccessToken"`
				VideoQualities []struct {
					FrameRate int    `json:"frameRate"`
					Quality   string `json:"quality"`
					SourceURL string `json:"sourceURL"`
				} `json:"videoQualities"`
			} `json:"clip"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := c.postGQL(ctx, payload, &response); err != nil {
		return nil, err
	}
	if len(response.Errors) > 0 {
		return nil, errors.New(response.Errors[0].Message)
	}
	if response.Data.Clip == nil {
		return nil, errors.New("clip not found or unavailable")
	}

	streams := map[string]Stream{}
	sig := response.Data.Clip.PlaybackAccessToken.Signature
	token := response.Data.Clip.PlaybackAccessToken.Value
	for _, quality := range response.Data.Clip.VideoQualities {
		if quality.SourceURL == "" {
			continue
		}
		name := quality.Quality + "p"
		if quality.FrameRate > 30 {
			name += strconv.Itoa(quality.FrameRate)
		}
		streams[name] = Stream{
			Name: name,
			Type: "http",
			URL:  addQuery(quality.SourceURL, map[string]string{"sig": sig, "token": token}),
		}
	}
	return streams, nil
}

func addQuery(rawURL string, values map[string]string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	for key, value := range values {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
