package main

import (
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type (
	story   []chapter
	chapter struct {
		title    string
		duration time.Duration
		// If allowNegativeObservations is false, any negative
		// observations will be performed as an observation of the value
		// zero.
		allowNegativeObservations bool
		sources                   []source
	}
	source struct {
		// TODO(beorn7): This is just a source of different latency
		// feeding into the same histogram so far. We could also have
		// the storyteller expose differently labeled histograms in the
		// future to demonstrate aggregation of histograms.
		durationBetweenRequestsMean   time.Duration
		durationBetweenRequestsStdDev time.Duration
		observedValueMean             float64
		observedValueStdDev           float64
	}
)

var (
	reg = prometheus.NewRegistry()
	his = promauto.With(reg).NewHistogram(prometheus.HistogramOpts{
		Name:                "story_observations",
		Help:                "Values observed during the story.",
		SparseBucketsFactor: 1.05,
	})
	aTaleOfLatencies = story{
		// This story is meant for a ~5min demo (but keeps running for
		// 9m20s without any changes in the last 5m). Scrape with 1s
		// scrape interval and do rates over 5s.
		{
			title:    "Low traffic",
			duration: time.Minute,
			sources: []source{
				{
					durationBetweenRequestsMean:   15 * time.Millisecond, // 67 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.25, // 250ms
					observedValueStdDev:           0.03,
				},
			},
		},
		{
			title:    "Increased traffic, with slightly increased latency",
			duration: time.Minute,
			sources: []source{
				{
					durationBetweenRequestsMean:   10 * time.Millisecond, // 100 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.3, // 300ms
					observedValueStdDev:           0.03,
				},
			},
		},
		{
			title:    "A successfull canary (10% of traffic), lower but less predictable latency",
			duration: 30 * time.Second,
			sources: []source{
				{
					durationBetweenRequestsMean:   11 * time.Millisecond, // 90 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.3, // 300ms
					observedValueStdDev:           0.03,
				},
				{
					durationBetweenRequestsMean:   100 * time.Millisecond, // 10 qps
					durationBetweenRequestsStdDev: 20 * time.Millisecond,
					observedValueMean:             0.20, // 200ms
					observedValueStdDev:           0.05,
				},
			},
		},
		{
			title:    "New version deployed everywhere",
			duration: 50 * time.Second,
			sources: []source{
				{
					durationBetweenRequestsMean:   10 * time.Millisecond, // 100 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.2, // 200ms
					observedValueStdDev:           0.05,
				},
			},
		},
		{
			title:    "One slow instance (out of 10) and one even slower",
			duration: 30 * time.Second,
			sources: []source{
				{
					durationBetweenRequestsMean:   13 * time.Millisecond, // 80 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.2, // 200ms
					observedValueStdDev:           0.05,
				},
				{
					durationBetweenRequestsMean:   100 * time.Millisecond, // 10 qps
					durationBetweenRequestsStdDev: 20 * time.Millisecond,
					observedValueMean:             0.4, // 400ms
					observedValueStdDev:           0.05,
				},
				{
					durationBetweenRequestsMean:   100 * time.Millisecond, // 10 qps
					durationBetweenRequestsStdDev: 20 * time.Millisecond,
					observedValueMean:             0.6, // 600ms
					observedValueStdDev:           0.05,
				},
			},
		},
		{
			title:    "Back to normal",
			duration: 10 * time.Second,
			sources: []source{
				{
					durationBetweenRequestsMean:   10 * time.Millisecond, // 100 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.2, // 200ms
					observedValueStdDev:           0.05,
				},
			},
		},
		{
			title:    "Everyone gets slower",
			duration: 10 * time.Second,
			sources: []source{
				{
					durationBetweenRequestsMean:   10 * time.Millisecond, // 100 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.25, // 250ms
					observedValueStdDev:           0.05,
				},
			},
		},
		{
			title:    "And slower",
			duration: 10 * time.Second,
			sources: []source{
				{
					durationBetweenRequestsMean:   10 * time.Millisecond, // 100 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.3, // 300ms
					observedValueStdDev:           0.06,
				},
			},
		},
		{
			title:    "And even slower",
			duration: 5 * time.Minute,
			sources: []source{
				{
					durationBetweenRequestsMean:   10 * time.Millisecond, // 100 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.35, // 350ms
					observedValueStdDev:           0.07,
				},
			},
		},
	}
)

func tellStory(s story) {
	for i, c := range s {
		log.Printf("Telling chapter %d: %s\n", i+1, c.title)
		end := make(chan struct{})
		time.AfterFunc(c.duration, func() { close(end) })
		for _, src := range c.sources {
			go func(src source, end chan struct{}) {
				for {
					d := time.Duration(
						rand.NormFloat64()*
							float64(src.durationBetweenRequestsStdDev) +
							float64(src.durationBetweenRequestsMean),
					)
					t := time.NewTimer(d)
					select {
					case <-t.C:
						o := rand.NormFloat64()*
							src.observedValueStdDev +
							src.observedValueMean
						if !c.allowNegativeObservations && o < 0 {
							o = 0
						}
						his.Observe(o)
					case <-end:
						t.Stop()
						return
					}
				}
			}(src, end)
		}
		<-end
	}
	log.Println("The end.")
}

func main() {
	go tellStory(aTaleOfLatencies)
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	log.Println("Serving metrics, SIGTERM to abort…")
	http.ListenAndServe(":8080", nil)
}
