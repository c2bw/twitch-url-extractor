package twitch

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"
)

// BenchmarkResult holds latency statistics from benchmarking URL extraction.
type BenchmarkResult struct {
	Iterations int
	Successes  int
	Failures   int
	Durations  []time.Duration
	Total      time.Duration
	Min        time.Duration
	Avg        time.Duration
	StdDev     time.Duration
	P50        time.Duration
	P90        time.Duration
	P95        time.Duration
	P99        time.Duration
	Max        time.Duration
	Streams    map[string]Stream
}

// Benchmark repeatedly extracts stream URLs for rawURL and returns latency statistics.
func (c *Client) Benchmark(ctx context.Context, rawURL string, iterations int) (BenchmarkResult, error) {
	if iterations <= 0 {
		return BenchmarkResult{}, errors.New("iterations must be greater than 0")
	}

	durations := make([]time.Duration, 0, iterations)
	var lastStreams map[string]Stream
	var lastErr error
	failures := 0

	startTotal := time.Now()
	for i := 0; i < iterations; i++ {
		if err := ctx.Err(); err != nil {
			return BenchmarkResult{}, err
		}

		start := time.Now()
		streams, err := c.Streams(ctx, rawURL)
		elapsed := time.Since(start)

		if err != nil {
			failures++
			lastErr = err
			continue
		}
		durations = append(durations, elapsed)
		lastStreams = streams
	}
	total := time.Since(startTotal)

	if len(durations) == 0 {
		if lastErr != nil {
			return BenchmarkResult{}, lastErr
		}
		return BenchmarkResult{}, errors.New("all benchmark iterations failed")
	}

	return ComputeBenchmarkStats(durations, failures, total, lastStreams), nil
}

// ComputeBenchmarkStats calculates min, average, standard deviation, and percentile
// latencies (p50, p90, p95, p99, max) from a slice of sample durations.
func ComputeBenchmarkStats(durations []time.Duration, failures int, total time.Duration, streams map[string]Stream) BenchmarkResult {
	n := len(durations)
	res := BenchmarkResult{
		Iterations: n + failures,
		Successes:  n,
		Failures:   failures,
		Durations:  append([]time.Duration(nil), durations...),
		Total:      total,
		Streams:    streams,
	}
	if n == 0 {
		return res
	}

	sorted := append([]time.Duration(nil), durations...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}
	avg := time.Duration(int64(sum) / int64(n))

	var varianceSum float64
	avgFloat := float64(avg)
	for _, d := range sorted {
		diff := float64(d) - avgFloat
		varianceSum += diff * diff
	}
	stdDev := time.Duration(math.Sqrt(varianceSum / float64(n)))

	res.Min = sorted[0]
	res.Max = sorted[n-1]
	res.Avg = avg
	res.StdDev = stdDev
	res.P50 = percentileDuration(sorted, 0.50)
	res.P90 = percentileDuration(sorted, 0.90)
	res.P95 = percentileDuration(sorted, 0.95)
	res.P99 = percentileDuration(sorted, 0.99)

	return res
}

func percentileDuration(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := p * float64(len(sorted)-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))
	if lower == upper {
		return sorted[lower]
	}
	weight := pos - float64(lower)
	interpolated := float64(sorted[lower])*(1-weight) + float64(sorted[upper])*weight
	return time.Duration(math.Round(interpolated))
}
