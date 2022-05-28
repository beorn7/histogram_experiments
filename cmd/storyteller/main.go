package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
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
		// If image is not nil, all other values are ignored. The x axis
		// of the image is mapped to the duration of the story, each x
		// pixel corresponding to the total duration divided by the
		// number of pixels along the x axis. Observations are then
		// conducted with a frequency according to the gray value of
		// each pixel at a given X value, with the observed value being
		// 2^(Y/maxY) and at most 255 observations per value.
		image image.Image
	}
)

var (
	reg = prometheus.NewRegistry()
	his = promauto.With(reg).NewHistogram(prometheus.HistogramOpts{
		Name:                "story_observations",
		Help:                "Values observed during the story.",
		SparseBucketsFactor: 1.01,
	})
	aTaleOfLatencies = story{
		// This story is meant for a ~5min demo (but keeps running for
		// 9m20s without any changes in the last 5m). Scrape with 1s
		// scrape interval and do rates over 5s.
		{
			title:    "Low traffic",
			duration: 45 * time.Second,
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
			title:    "Transition to increased traffic, with slightly increased latency",
			duration: 15 * time.Second,
			sources: []source{
				{
					durationBetweenRequestsMean:   13 * time.Millisecond, // 100 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.27,
					observedValueStdDev:           0.03,
				},
			},
		},
		{
			title:    "Transition to increased traffic, with slightly increased latency",
			duration: 15 * time.Second,
			sources: []source{
				{
					durationBetweenRequestsMean:   11 * time.Millisecond, // 100 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.29,
					observedValueStdDev:           0.03,
				},
			},
		},
		{
			title:    "Increased traffic, with slightly increased latency",
			duration: 45 * time.Second,
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
			title:    "A successfull canary (20% of traffic), lower but less predictable latency",
			duration: 30 * time.Second,
			sources: []source{
				{
					durationBetweenRequestsMean:   13 * time.Millisecond, // 80 qps
					durationBetweenRequestsStdDev: 2 * time.Millisecond,
					observedValueMean:             0.3, // 300ms
					observedValueStdDev:           0.03,
				},
				{
					durationBetweenRequestsMean:   50 * time.Millisecond, // 20 qps
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
			go src.run(end, c.duration, c.allowNegativeObservations)
		}
		<-end
	}
	log.Println("The end.")
}

func (src source) run(end chan struct{}, duration time.Duration, allowNegativeObservations bool) {
	if src.image != nil {
		src.runImage(duration)
		return
	}
	src.runNorm(end, allowNegativeObservations)
}

func (src source) runNorm(end chan struct{}, allowNegativeObservations bool) {
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
			if !allowNegativeObservations && o < 0 {
				o = 0
			}
			his.Observe(o)
		case <-end:
			t.Stop()
			return
		}
	}
}

func (src source) runImage(duration time.Duration) {
	bounds := src.image.Bounds()
	ticker := time.NewTicker(duration / time.Duration(bounds.Max.X-bounds.Min.X))
	defer ticker.Stop()
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			pixel := src.image.At(x, y)
			gray := color.GrayModel.Convert(pixel).(color.Gray).Y
			value := math.Exp2((float64(bounds.Max.Y-y) - rand.Float64()) / float64(bounds.Max.Y))
			for i := gray; i > 0; i-- {
				his.Observe(value)
			}
		}
		<-ticker.C
	}
}

func main() {
	// go tellStory(aTaleOfLatencies)

	f, err := os.Open("./prometheus.png")
	// f, err := os.Open("./grafana.png")
	// f, err := os.Open("./rickroll.png")
	if err != nil {
		log.Fatal(err)
	}
	promImage, err := png.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	prometheusLogo := story{
		chapter{
			title:    "The Prometheus logo",
			duration: 5 * time.Minute,
			sources: []source{
				{
					image: promImage,
				},
			},
		},
	}

	go tellStory(prometheusLogo)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	log.Println("Serving metrics, SIGTERM to abort…")
	http.ListenAndServe(":8080", nil)
}
