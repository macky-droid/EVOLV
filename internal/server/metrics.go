package server

import (
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
	"time"
)

// latencyHistogram is a fixed-bucket Prometheus histogram without external
// dependencies. Buckets are in seconds; observations are cumulative on write.
type latencyHistogram struct {
	buckets [11]atomic.Uint64
	count   atomic.Uint64
	sumUS   atomic.Uint64
}

var latencyBounds = [11]float64{
	0.005, 0.010, 0.025, 0.050, 0.100, 0.250,
	0.500, 1.000, 2.500, 5.000, 10.000,
}

func (h *latencyHistogram) observe(d time.Duration) {
	seconds := d.Seconds()
	for i, bound := range latencyBounds {
		if seconds <= bound {
			h.buckets[i].Add(1)
			break
		}
	}
	h.count.Add(1)
	h.sumUS.Add(uint64(d.Microseconds()))
}

func (h *latencyHistogram) writePrometheus(w io.Writer, name string) {
	fmt.Fprintf(w, "# HELP %s HTTP request latency in seconds.\n", name)
	fmt.Fprintf(w, "# TYPE %s histogram\n", name)
	var cumulative uint64
	for i, bound := range latencyBounds {
		cumulative += h.buckets[i].Load()
		fmt.Fprintf(w, "%s_bucket{le=%q} %d\n", name, strconv.FormatFloat(bound, 'g', -1, 64), cumulative)
	}
	fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n", name, h.count.Load())
	fmt.Fprintf(w, "%s_sum %g\n", name, float64(h.sumUS.Load())/1e6)
	fmt.Fprintf(w, "%s_count %d\n", name, h.count.Load())
}
