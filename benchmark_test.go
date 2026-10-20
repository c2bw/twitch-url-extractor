package twitch

import (
	"testing"
	"time"
)

func TestComputeBenchmarkStats(t *testing.T) {
	samples := []time.Duration{
		100 * time.Millisecond,
		110 * time.Millisecond,
		120 * time.Millisecond,
		130 * time.Millisecond,
		140 * time.Millisecond,
		150 * time.Millisecond,
		160 * time.Millisecond,
		170 * time.Millisecond,
		180 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
	}

	stats := ComputeBenchmarkStats(samples, 1, 2*time.Second, nil)

	if stats.Iterations != 12 || stats.Successes != 11 || stats.Failures != 1 {
		t.Fatalf("counts = %+v", stats)
	}
	if stats.Min != 100*time.Millisecond {
		t.Fatalf("Min = %v, want 100ms", stats.Min)
	}
	if stats.Max != 300*time.Millisecond {
		t.Fatalf("Max = %v, want 300ms", stats.Max)
	}
	if stats.Avg != 160*time.Millisecond {
		t.Fatalf("Avg = %v, want 160ms", stats.Avg)
	}
	if stats.P50 != 150*time.Millisecond {
		t.Fatalf("P50 = %v, want 150ms", stats.P50)
	}
	if stats.P90 != 200*time.Millisecond {
		t.Fatalf("P90 = %v, want 200ms", stats.P90)
	}
	if stats.P95 != 250*time.Millisecond {
		t.Fatalf("P95 = %v, want 250ms", stats.P95)
	}
}
