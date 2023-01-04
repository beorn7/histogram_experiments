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

	reg = prometheus.NewRegistry()
)

func observe() {
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

	iH := *integerCounter.Metric[0].Histogram
	fH := iH
	fH.SampleCountFloat = proto.Float64(float64(*iH.SampleCount) / 2)
	fH.SampleCount = nil
	fH.SampleSum = proto.Float64(*iH.SampleSum / 2)

	metric := *integerCounter.Metric[0]
	metrics := []*dto.Metric{&metric}
	metric.Histogram = &fH

	// TODO adjust buckets

	floatCounter.Metric = metrics
	floatGauge.Metric = metrics

	mfs = append(mfs, &floatCounter, &floatGauge, &integerGauge)
	return mfs, nil
}

func main() {
	flag.Parse()
	http.Handle("/metrics", promhttp.HandlerFor(&WrappingGatherer{reg}, promhttp.HandlerOpts{}))

	go observe()

	log.Println("Serving metrics, SIGTERM to abort…")
	http.ListenAndServe(*addr, nil)
}
