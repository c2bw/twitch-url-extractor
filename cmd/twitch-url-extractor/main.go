package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	twitch "github.com/c2bw/twitch-url-extractor"
)

func main() {
	fs := flag.NewFlagSet("twitch-url-extractor", flag.ExitOnError)
	benchmark := fs.Bool("benchmark", false, "Run benchmark mode to measure extraction latency (avg, p50, p90, p95, p99)")
	fs.BoolVar(benchmark, "b", false, "Shorthand for --benchmark")
	iterations := fs.Int("n", 10, "Number of benchmark iterations")
	fs.IntVar(iterations, "iterations", 10, "Number of benchmark iterations")
	warmup := fs.Int("warmup", 1, "Number of warmup requests before benchmarking")
	timeout := fs.Duration("timeout", 15*time.Second, "Timeout per extraction request")
	codecs := fs.String("codecs", "h264", "Comma-separated list of supported codecs (e.g. h264,h265,av1)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <twitch-url>\n\nFlags:\n", os.Args[0])
		fs.PrintDefaults()
	}

	args, rawURL := splitArgs(os.Args[1:])
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if rawURL == "" && fs.NArg() > 0 {
		rawURL = fs.Arg(0)
	}
	if rawURL == "" {
		fs.Usage()
		os.Exit(1)
	}

	client := twitch.NewClient(nil)
	if *codecs != "" {
		var list []string
		for _, c := range strings.Split(*codecs, ",") {
			if c = strings.TrimSpace(c); c != "" {
				list = append(list, c)
			}
		}
		client.SupportedCodecs = list
	}

	if !*benchmark {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()

		streams, err := client.Streams(ctx, rawURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		printStreams(streams)
		return
	}

	runBenchmark(client, rawURL, *iterations, *warmup, *timeout)
}

func runBenchmark(client *twitch.Client, rawURL string, iterations, warmup int, timeout time.Duration) {
	if iterations <= 0 {
		fmt.Fprintln(os.Stderr, "error: iterations (-n) must be > 0")
		os.Exit(1)
	}

	fmt.Printf("Target URL : %s\n", rawURL)
	fmt.Printf("Codecs     : %s\n", strings.Join(client.SupportedCodecs, ","))
	fmt.Printf("Iterations : %d (warmup: %d)\n\n", iterations, warmup)

	for i := 0; i < warmup; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		_, err := client.Streams(ctx, rawURL)
		elapsed := time.Since(start)
		cancel()
		if err != nil {
			fmt.Printf("  [warmup %d/%d] FAILED (%s): %v\n", i+1, warmup, formatDuration(elapsed), err)
		} else {
			fmt.Printf("  [warmup %d/%d] %s\n", i+1, warmup, formatDuration(elapsed))
		}
	}

	durations := make([]time.Duration, 0, iterations)
	var lastStreams map[string]twitch.Stream
	var lastErr error
	failures := 0

	startTotal := time.Now()
	for i := 0; i < iterations; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		streams, err := client.Streams(ctx, rawURL)
		elapsed := time.Since(start)
		cancel()

		if err != nil {
			failures++
			lastErr = err
			fmt.Printf("  [run %2d/%d] FAILED (%s): %v\n", i+1, iterations, formatDuration(elapsed), err)
			continue
		}
		durations = append(durations, elapsed)
		lastStreams = streams
		fmt.Printf("  [run %2d/%d] %s (%d streams)\n", i+1, iterations, formatDuration(elapsed), len(streams))
	}
	total := time.Since(startTotal)

	if len(durations) == 0 {
		fmt.Fprintf(os.Stderr, "\nAll benchmark iterations failed. Last error: %v\n", lastErr)
		os.Exit(1)
	}

	stats := twitch.ComputeBenchmarkStats(durations, failures, total, lastStreams)

	fmt.Println("\n--- Benchmark Results ---")
	fmt.Printf("Requests   : %d total, %d ok, %d failed\n", stats.Iterations, stats.Successes, stats.Failures)
	fmt.Printf("Total Time : %s\n", formatDuration(stats.Total))
	fmt.Printf("Min        : %s\n", formatDuration(stats.Min))
	fmt.Printf("Avg (Mean) : %s\n", formatDuration(stats.Avg))
	fmt.Printf("StdDev     : %s\n", formatDuration(stats.StdDev))
	fmt.Printf("p50        : %s\n", formatDuration(stats.P50))
	fmt.Printf("p90        : %s\n", formatDuration(stats.P90))
	fmt.Printf("p95        : %s\n", formatDuration(stats.P95))
	fmt.Printf("p99        : %s\n", formatDuration(stats.P99))
	fmt.Printf("Max        : %s\n", formatDuration(stats.Max))

	if best, ok := twitch.BestStream(stats.Streams); ok {
		fmt.Printf("\nBest Stream: %s (%s)\n", best.Name, best.Type)
	}
}

func printStreams(streams map[string]twitch.Stream) {
	names := make([]string, 0, len(streams))
	for name := range streams {
		names = append(names, name)
	}
	sort.Strings(names)

	if best, ok := twitch.BestStream(streams); ok {
		fmt.Printf("best (%s): %s\n\n", best.Name, best.URL)
	}
	for _, name := range names {
		fmt.Printf("%s: %s\n", name, streams[name].URL)
	}
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000.0)
}

// splitArgs allows flags to be placed either before or after the positional URL argument.
func splitArgs(raw []string) (flags []string, positional string) {
	takesValue := map[string]bool{
		"-n":           true,
		"--n":          true,
		"-iterations":  true,
		"--iterations": true,
		"-warmup":      true,
		"--warmup":     true,
		"-timeout":     true,
		"--timeout":    true,
		"-codecs":      true,
		"--codecs":     true,
	}

	for i := 0; i < len(raw); i++ {
		arg := raw[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			if takesValue[arg] && !strings.Contains(arg, "=") && i+1 < len(raw) {
				i++
				flags = append(flags, raw[i])
			}
		} else if positional == "" {
			positional = arg
		} else {
			flags = append(flags, arg)
		}
	}
	return flags, positional
}
