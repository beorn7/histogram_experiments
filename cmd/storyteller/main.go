package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type (
	story   []chapter
	chapter struct {
		title    string
		duration time.Duration
		sources  []source
	}
	source struct {
		durationBetweenRequestsMean   time.Duration
		durationBetweenRequestsStdDev float64
		observedValueMean             float64
		observedValueStdDev           float64
	}
)

var (
	his = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:                "story_observations",
		Help:                "Values observed during the story.",
		SparseBucketsFactor: 1.1,
	})
	aTaleOfLatencies = story{
		{
			title:    "Low Traffic",
			duration: time.Minute,
		},
		{
			title:    "Increased Traffic",
			duration: time.Minute,
		},
		{
			title:    "A Successfull Canary",
			duration: 30 * time.Second,
		},
		{
			title:    "New Version Deployed Everywhere",
			duration: 50 * time.Second,
		},
		{
			title:    "One Slow Instance And One Even Slower",
			duration: 30 * time.Second,
		},
		{
			title:    "Back to Normal",
			duration: 10 * time.Second,
		},
		{
			title:    "Everyone Gets Slower",
			duration: 10 * time.Second,
		},
		{
			title:    "And Slower",
			duration: 10 * time.Second,
		},
		{
			title:    "And even Slower",
			duration: 5 * time.Minute,
		},
	}
)

func tellStory(s story) {
	for i, c := range s {
		fmt.Printf("Telling chapter %d: %s\n", i+1, c.title)
	}
	fmt.Println("The end.")
}

func main() {
	go tellStory(aTaleOfLatencies)
	log.Println("Serving metrics, SIGTERM to abort…")
	http.ListenAndServe(":8080", nil)
}
