package twitch

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	attrRE       = regexp.MustCompile(`([A-Z0-9-]+)=("[^"]*"|[^,]*)`)
	resolutionRE = regexp.MustCompile(`RESOLUTION=\d+x(\d+)`)
	frameRateRE  = regexp.MustCompile(`FRAME-RATE=([0-9.]+)`)
	bandwidthRE  = regexp.MustCompile(`BANDWIDTH=(\d+)`)
	qualityRE    = regexp.MustCompile(`(\d+)p(?:(\d+))?`)
	offsetRE     = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)(h|m|s)`)
)

func parseVariantPlaylist(baseURL, playlist string, offset time.Duration) map[string]Stream {
	streams := map[string]Stream{}
	lines := strings.Split(playlist, "\n")

	mediaNames := map[string]string{}
	var lastInf string

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "#EXT-X-MEDIA:"):
			attrs := parseAttrs(strings.TrimPrefix(line, "#EXT-X-MEDIA:"))
			if attrs["TYPE"] == "VIDEO" && attrs["GROUP-ID"] != "" && attrs["NAME"] != "" {
				mediaNames[attrs["GROUP-ID"]] = attrs["NAME"]
			}
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF:"):
			lastInf = strings.TrimPrefix(line, "#EXT-X-STREAM-INF:")
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case lastInf != "":
			name := streamName(lastInf, mediaNames)
			if name == "" {
				name = fmt.Sprintf("%dk", len(streams)+1)
			}
			name = uniqueName(streams, name)
			streams[name] = Stream{
				Name: name,
				Type: "hls",
				URL:  withStartOffset(resolveURL(baseURL, line), offset),
			}
			lastInf = ""
		}
	}

	return streams
}

func parseAttrs(value string) map[string]string {
	attrs := map[string]string{}
	for _, match := range attrRE.FindAllStringSubmatch(value, -1) {
		attrs[match[1]] = strings.Trim(match[2], `"`)
	}
	return attrs
}

func streamName(streamInf string, mediaNames map[string]string) string {
	attrs := parseAttrs(streamInf)
	if video := attrs["VIDEO"]; video != "" {
		if name := mediaNames[video]; name != "" {
			if strings.EqualFold(name, "audio") {
				return "audio_only"
			}
			if strings.EqualFold(name, "chunked") {
				return "source"
			}
			return name
		}
		if strings.EqualFold(video, "chunked") {
			return "source"
		}
		if strings.Contains(strings.ToLower(video), "audio") {
			return "audio_only"
		}
	}

	if match := resolutionRE.FindStringSubmatch(streamInf); match != nil {
		name := match[1] + "p"
		if fpsMatch := frameRateRE.FindStringSubmatch(streamInf); fpsMatch != nil {
			fps, _ := strconv.ParseFloat(fpsMatch[1], 64)
			if fps > 30 {
				name += fmt.Sprint(int(math.Ceil(fps)))
			}
		}
		return name
	}

	if match := bandwidthRE.FindStringSubmatch(streamInf); match != nil {
		bw, _ := strconv.Atoi(match[1])
		if bw >= 1000 {
			return fmt.Sprintf("%dk", bw/1000)
		}
		return fmt.Sprintf("%.1fk", float64(bw)/1000)
	}

	return ""
}

func uniqueName(streams map[string]Stream, name string) string {
	if _, exists := streams[name]; !exists {
		return name
	}
	for idx := 1; idx <= 2; idx++ {
		candidate := fmt.Sprintf("%s_alt%d", name, idx)
		if _, exists := streams[candidate]; !exists {
			return candidate
		}
	}
	return fmt.Sprintf("%s_alt%d", name, len(streams)+1)
}

func resolveURL(baseURL, ref string) string {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return ref
	}
	parsedRef, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return parsedBase.ResolveReference(parsedRef).String()
}

func withStartOffset(rawURL string, offset time.Duration) string {
	if offset <= 0 {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	q.Set("start_offset", fmt.Sprint(int(offset.Seconds())))
	u.RawQuery = q.Encode()
	return u.String()
}

func parseOffset(value string) time.Duration {
	if value == "" {
		return 0
	}

	matches := offsetRE.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return 0
	}

	var total float64
	for _, match := range matches {
		n, _ := strconv.ParseFloat(match[1], 64)
		switch strings.ToLower(match[2]) {
		case "h":
			total += n * 3600
		case "m":
			total += n * 60
		case "s":
			total += n
		}
	}
	return time.Duration(total * float64(time.Second))
}

func streamWeight(name string) int {
	if name == "source" {
		return 1_000_000
	}
	if strings.HasPrefix(name, "audio") {
		return 0
	}
	if match := qualityRE.FindStringSubmatch(name); match != nil {
		height, _ := strconv.Atoi(match[1])
		fps, _ := strconv.Atoi(match[2])
		return height*1000 + fps
	}
	if strings.HasSuffix(name, "k") {
		n, _ := strconv.Atoi(strings.TrimSuffix(name, "k"))
		return n
	}
	return 1
}

func BestStream(streams map[string]Stream) (Stream, bool) {
	var best Stream
	var ok bool
	var bestWeight int
	for name, stream := range streams {
		if stream.URL == "" {
			continue
		}
		weight := streamWeight(name)
		if !ok || weight > bestWeight {
			best = stream
			ok = true
			bestWeight = weight
		}
	}
	return best, ok
}
