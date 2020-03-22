# Histogram Experiments

To play with new approaches to represent histograms in Prometheus, this
repository provides two tools:

1. `exposer`: Reads in observations from a dataset and exposes them in a
   histogram.
2. `scraper`: Scrapes a target with histograms (usally `exposer`) and prints
   out stats about it.

## The datasets

Datasets are in the `datasets` directory:

- `cortex-querier.1h.20200309`: 1h (from 16:00 to 17:00 UTC) of query latencies
  from Cortex queriers at Grafana Labs. The file contains 668,925 observations
  (186 qps on average). Those queries vary a lot in cost and therefore have a
  relatively wide spread of latencies. They are expected to cover a lot of
  different histogram buckets and thus to represent one of the more expensive
  real-world use cases.
- TODO: A dataset with a relatively sharp peak from a service where each
  request has approximately the same cost, ideally with an “outage” included
  where some of the requests have a higher latency for a while.
- TODO: SpamAssassin scores from my mailserver. The observation here are
  rounded to one decimal place, but they can be negative.
