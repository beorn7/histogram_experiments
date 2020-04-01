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
- `cortex-ingester.2m.20200326`: 2m (from 20:00 to 20:02 UTC) of processing
  latencies from Cortex ingesters at Grafana Labs. The file contains 1,876,573
  observations (16k qps on average). Since an ingestion is relatively uniform
  in cost, the latencies have more narrow spread. In the only 2m the
  observations were taken from, the system operated normally. During degraded
  performance, the distribution of latencies would become less sharp.
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

The `cortex-ingester.2m.20200326` dataset has almost 3x the number of
observations, but uses only 78 buckets with a quite sharp maximum at index –78
(approx. 112µs < x ≤ 126µs, 280941 observations). The buckets range from index
–91 to –13, with all buckets in between except the one at index –15 having at
least one observations. The largest count difference between consecutive
buckets is 70867 (between index –81 and –80).

Again, encoding the schema of used buckets is efficient as spans (two spans in
this case: 76 consecutive buckets from –91, 2 consecutive buckets from
–14). The largest delta between buckets needs 3 bytes as a varint, but even
with the high observation count here, most deltas (45 out of 78) would fit into
a 1-byte varint (between –64 and 63). The total histogram is thus encodable in
a varint-based delta encoding in less than 150 bytes.

On my laptop, it took the exposer 550ms to read in the dataset, parse it, and
perform all the observations (300ns/observation). That's pretty decent for the
ad-hoc code in client_golang.
