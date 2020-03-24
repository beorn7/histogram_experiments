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
  
## Observations

With the very broad `cortex-querier.1h.20200309` dataset and a resolution (per
power of 10) of 20, the resulting sparse histogram has 112 buckets, from index
–80 (approx. 89.1µs < x ≤ 100µs) to index 31 (approx. 31.6s < x ≤ 35.5s). Every
bucket in this range has at least one entry. The largest count difference
between consecutive buckets is 9240 (between index –54 and –53). The most
populated bucket is index –48 (3.55ms < x ≤ 3.98ms, 43335 observations).

From that observation, specifying the used buckets as spans of consecutively
used buckets appears very efficient (in this case, it boils down to only one
span: 112 buckets starting at index –80, encodable in 1 byte of uvarint and 2
bytes of varint).

Furthermore, 56 of the 112 buckets have a delta to their previous bucket that
fits into a 1-byte varint (between –64 and 63). The largest delta (9240)
requires a 3-byte varint. Since a 2-byte varint encodes ranges from –8192 to
8191, all bucket counts (and in fact the whole histogram) are encodable in a
varint-based delta encoding in less than 200 bytes.
