package main

import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	dto "github.com/prometheus/client_model/go"
)

var (
	addr          = flag.String("listen-address", ":8080", "address to listen on for HTTP requests")
	factor        = flag.Float64("factor", 1.1, "each bucket is by this factor wider than the previous one, must be greater 1")
	zeroThreshold = flag.Float64("zero-threshold", 0.1, "width of the “zero” bucket")
)

func observe(reg prometheus.Registerer) {
	// TODO: simulate occasional counter resets.
	var (
		his = promauto.With(reg).NewHistogram(prometheus.HistogramOpts{
			Name:                         "integer_counter_histogram",
			Help:                         "Test histogram for an experiment.",
			NativeHistogramBucketFactor:  *factor,
			NativeHistogramZeroThreshold: *zeroThreshold,
		})
	)

	for {
		his.Observe(rand.NormFloat64())
		time.Sleep(time.Duration(rand.Int31n(100)) * time.Millisecond)
	}
}

type WrappingGatherer struct {
	prometheus.Gatherer
}

func (g *WrappingGatherer) Gather() ([]*dto.MetricFamily, error) {
	mfs, err := g.Gatherer.Gather()
	if err != nil {
		return mfs, err
	}
	integerCounter := *mfs[0]
	floatCounter, floatGauge, integerGauge := integerCounter, integerCounter, integerCounter

	floatCounter.Name = proto.String("float_counter_histogram")
	floatGauge.Name = proto.String("float_gauge_histogram")
	integerGauge.Name = proto.String("integer_gauge_histogram")

	floatGauge.Type = dto.MetricType_GAUGE_HISTOGRAM.Enum()
	integerGauge.Type = dto.MetricType_GAUGE_HISTOGRAM.Enum()

	fH := *integerCounter.Metric[0].Histogram
	fH.SampleCountFloat = proto.Float64(float64(*fH.SampleCount) / 2)
	fH.SampleCount = nil
	fH.SampleSum = proto.Float64(*fH.SampleSum / 2)
	fH.ZeroCountFloat = proto.Float64(float64(*fH.ZeroCount) / 2)
	fH.ZeroCount = nil

	var currentCount int64
	for _, d := range fH.PositiveDelta {
		currentCount += d
		fH.PositiveCount = append(fH.PositiveCount, float64(currentCount)/2)
	}
	fH.PositiveDelta = nil
	currentCount = 0
	for _, d := range fH.NegativeDelta {
		currentCount += d
		fH.NegativeCount = append(fH.NegativeCount, float64(currentCount)/2)
	}
	fH.NegativeDelta = nil

	metric := *integerCounter.Metric[0]
	metrics := []*dto.Metric{&metric}
	metric.Histogram = &fH

	floatCounter.Metric = metrics
	floatGauge.Metric = metrics

	mfs = append(mfs, &floatCounter, &floatGauge, &integerGauge)
	return mfs, nil
}

func main() {
	flag.Parse()

	reg := prometheus.NewRegistry()
	http.Handle("/metrics", promhttp.HandlerFor(&WrappingGatherer{reg}, promhttp.HandlerOpts{}))

	go observe(reg)

	log.Println("Serving metrics, SIGTERM to abort…")
	http.ListenAndServe(*addr, nil)
}
