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
	qualityRE    = regexp.MustCompile(`^(\d+)p(?:(\d+))?$`)
	altRE        = regexp.MustCompile(`_alt(\d*)$`)
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
	pixelsWithFPS := pixelsName(streamInf, true)
	variantID := strings.ToLower(attrs["STABLE-VARIANT-ID"])
	ivsGroups := strings.ToLower(attrs["IVS-GROUPS"])

	if pixelsWithFPS != "" && ivsGroups == "portrait" {
		return pixelsWithFPS + "_portrait"
	}
	if strings.HasPrefix(variantID, "audio") {
		return "audio_only"
	}
	if variantID != "" {
		return variantID
	}
	if ivsName := attrs["IVS-NAME"]; ivsName != "" {
		lowerIVS := strings.ToLower(ivsName)
		if lowerIVS == "audio_only" || lowerIVS == "audio" || lowerIVS == "audio only" {
			return "audio_only"
		}
		return ivsName
	}

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

	if name := pixelsName(streamInf, false); name != "" {
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

func pixelsName(streamInf string, forceFramerate bool) string {
	match := resolutionRE.FindStringSubmatch(streamInf)
	if match == nil || match[1] == "0" {
		return ""
	}
	name := match[1] + "p"
	if fpsMatch := frameRateRE.FindStringSubmatch(streamInf); fpsMatch != nil {
		fps, err := strconv.ParseFloat(fpsMatch[1], 64)
		if err == nil && (forceFramerate || fps > 30) {
			name += fmt.Sprint(int(math.Ceil(fps)))
		}
	}
	return name
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
	if strings.HasPrefix(name, "audio") {
		return 0
	}

	baseName := name
	altPenalty := 0
	if match := altRE.FindStringSubmatch(baseName); match != nil {
		altPenalty = 1
		if match[1] != "" {
			if n, err := strconv.Atoi(match[1]); err == nil && n > 0 {
				altPenalty = n
			}
		}
		baseName = baseName[:len(baseName)-len(match[0])]
	}

	isPortrait := strings.HasSuffix(baseName, "_portrait")
	baseName = strings.TrimSuffix(baseName, "_portrait")

	weight := 10
	switch {
	case baseName == "source":
		weight = 90_000_000
	case qualityRE.MatchString(baseName):
		match := qualityRE.FindStringSubmatch(baseName)
		height, _ := strconv.Atoi(match[1])
		fps, _ := strconv.Atoi(match[2])
		weight = (height*1000 + fps) * 10
	case strings.HasSuffix(baseName, "k"):
		f, _ := strconv.ParseFloat(strings.TrimSuffix(baseName, "k"), 64)
		weight = int(f) * 10
	}

	weight -= altPenalty
	if isPortrait {
		return weight
	}
	return 100_000_000 + weight
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
		if !ok || weight > bestWeight || (weight == bestWeight && name < best.Name) {
			best = stream
			ok = true
			bestWeight = weight
		}
	}
	return best, ok
}
