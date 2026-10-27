package auth_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"os"
	"sort"
	"testing"
	"time"
)

// Opt-in measurement, not a timing assertion: infrastructure jitter must not
// turn ordinary regression tests flaky. No addresses, tokens, or secrets log.
func TestPasswordResetRequestTiming(t *testing.T) {
	if os.Getenv("AUMA_MEASURE_RESET_TIMING") != "1" {
		t.Skip("opt-in DB timing measurement")
	}
	u := createTestUser(t, 2)
	service := resetService()
	ctx := context.Background()
	missing := fmt.Sprintf("missing-timing-%d@example.com", time.Now().UnixNano())
	measure := func(email string) float64 {
		start := time.Now()
		if err := service.Request(ctx, email); err != nil {
			t.Fatal(err)
		}
		return float64(time.Since(start)) / float64(time.Millisecond)
	}
	for i := 0; i < 5; i++ {
		measure(u.Email)
		measure(missing)
	}
	const count = 100
	existing, absent := make([]float64, 0, count), make([]float64, 0, count)
	wins := 0
	for i := 0; i < count; i++ {
		var choice [1]byte
		if _, err := rand.Read(choice[:]); err != nil {
			t.Fatal(err)
		}
		var a, b float64
		if choice[0]&1 == 0 {
			a = measure(u.Email)
			b = measure(missing)
		} else {
			b = measure(missing)
			a = measure(u.Email)
		}
		existing = append(existing, a)
		absent = append(absent, b)
		if a > b {
			wins++
		}
	}
	stats := func(v []float64) (mean, sd, median, p95 float64) {
		for _, x := range v {
			mean += x
		}
		mean /= float64(len(v))
		for _, x := range v {
			sd += (x - mean) * (x - mean)
		}
		sd = math.Sqrt(sd / float64(len(v)-1))
		sorted := append([]float64(nil), v...)
		sort.Float64s(sorted)
		return mean, sd, (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2, sorted[int(float64(len(sorted))*.95)-1]
	}
	am, as, amed, ap95 := stats(existing)
	bm, bs, bmed, bp95 := stats(absent)
	t.Logf("local DB service (rate-limit bypassed for measurement), n=%d per group; existing mean=%.3f sd=%.3f median=%.3f p95=%.3f ms; absent mean=%.3f sd=%.3f median=%.3f p95=%.3f ms; existing slower in %d/%d paired samples", count, am, as, amed, ap95, bm, bs, bmed, bp95, wins, count)
}
