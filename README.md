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
- `spamd.20190918`: SpamAssassin scores from 21,761 mails, collected between
  2019-09-18 and 2020-04-01 (from the small mailserver I run for my
  family). The observed values here are rounded to one decimal place, but they can
  be negative. The distribution is fairly irregular. This dataset is intended
  to test how well the histogram works with an atypical distribution, not
  related to the usual request latency measurement.
  
## The basic idea

The basic idea is to have an “infinite buckets” histogram with a regular
logarithmic (or logarithmic-linear) bucketing schema and efficient handling of
sparseness (i.e. empty buckets don't take any resources). This idea has been
used in other implementations for quite some time, e.g. HDR Histogram,
circlllhist, more recently DDSketch, nicely described and compared in [a recent
paper by Heinrich Hartmann and Theo
Schlossnagle](https://arxiv.org/abs/2001.06561). Such an approach is
conceptionally much simpler than a compressed “digest” approach, for which
there is a huge body of research to study. A digest approach is probably not
feasible for the Prometheus context. Even a “sparse histogram” approach has to
cope with certain Prometheus properties, see [my PromCon 2019
talk](https://promcon.io/2019-munich/talks/prometheus-histograms-past-present-and-future/)
for an overview. This repository is essentially a playground to study how such
an approach could work for Prometheus.

The experimental approach so far is broadly the following:

- Define a configurable “resolution”: the number of buckets per power of 10. So
  far, a value of 20 seems to be a good trade-off. Note that keeping this
  number the same over time allows merging of histograms, while changing it
  disrupts that. The buckets are then spaced logarithmically, resulting in
  “weird” bucket boundaries, but every power of 10 will also be a buckte
  boundary (10ms, 100ms, 1s, 10s, …). (The circlllhist idea of spacing buckets
  linearly within a power of 10 to have “round” numbers could easily be
  implemented in a very similar way.)
- The bucketing schema is mirrored for negative observations.
- Define a “zero bucket” with a fixed width. All observations with an absolute
  number below that threshold fall into that “zero bucket”.

A proof-of-concept implementation for the instrumentation side can be found in
[a branch of
`prometheus/client_golang`](https://github.com/prometheus/client_golang/tree/beorn7/histogram).
The implementation is very straight-forward using `sync.Map`. I assume it to be
reasonably efficient, but there is probably a lot of room for
improvement. Inspiration can be taken from open-source implementations of
similar concepts, see above.

The challenge of the Prometheus use case is that a monitored target cannot just
collect observations for a fixed time frame and then sent it off for good to
the metrics collection system. With Prometheus, monitored targets accumulate
counts essentially forever and accept “stateless” scrapes from any number of
scrapers. The assumption is, however, that with real-world observations, the
number of used buckets will quickly approach a not too large number, so that
resetting the histogram every minute is actually not so much different from
never resetting it. The datasets provided in this repo are used to provide
evidence for that assumption, see observations below.

Having said that, a very occasional (i.e. rare compared to the scrape interval)
reset of a histogram is relatively harmless. For example, we could simply reset
all histograms every hour, creating a relatively small loss of counts. That's
particularly helpful if there was a short time span during which “exotic”
buckets were populated, e.g. during degraded performance.

Another approach to limiting the bucket count would be to dynamically widen the
“zero bucket”, which is essentially what DDSketch is doing.
  
## The exposition format

A [branch of
`prometheus/client_model`](https://github.com/prometheus/client_model/tree/beorn7/histogram),
contains an experimental protobuf definition for sparse buckets as an
additional or alternative bucket set within a conventional Prometheus
Histogram.

- The resolution (number of buckets for each power of 10) is limited to 255 in
  the PoC implementation in `prometheus/client_golang`. There is no `uint8`
  data type in protobuf, so `sb_resolution` is defined as a `uint32`. Most
  reasonable resolutions should only take 1 byte in that way. A missing value
  or a value of zero signals that no sparse buckets are used.
- The breadth of the “zero bucket” is a `double` called `sb_zero_threshold`.
- The count of observations in the “zero bucket” is a `uint64` called
  `sb_zero_threshold`.
- Sparse buckets for negative and positive buckets are then represented in a
  `SparseBuckets` message each. If there are no negative or positive
  observations, the respective message is left empty.
- Each `SpareBuckets` message contains the following:
  - The counts in the buckets in a delta-encoding, i.e. each value is the
    difference to the previous bucket (or to zero in case of the first bucket),
    assuming that those differences are smaller than absolute numbers in most
    real-world use cases. The type is `sint64`, which uses varint encoding in a
    way that's efficient for negative numbers, too. (The deltas are expected to
    be negative about as often as positive.) In usual benign use cases, most of
    the count deltas should fit into a single byte.
  - The “schema” of the buckets is encoded separately in one or more spans of
    consecutive buckets. Each span contains the `offset` to the previous span
    (or simply the starting bucket index for the first span) and then the
    number of consecutive buckets (the `length` of the span). Neither uses any
    tricks with zero-basing to make it easier to read and understand, at the
    cost of very few additional bits, i.e. a value of 0 for the `offset` will
    never occur except possibly for the 1st span (a span with `offset` 0 didn't
    need to be a separate span), and likewise a span with `length` 0 would not
    make sense. With the span encoding, histograms with mostly consecutive
    buckets are easy to describe. As can be seen from the observations below,
    real-world histograms have a lot of consecutive histograms.
	
The buckets are indexed around 0, where 0 stands for the bucket with an upper
bound of 1 (or a lower bound of –1 in the case of negative buckets). From
there, consecutive buckets are just counted up and down, so that each index
increment by the resolution value corresponds to a power of 10.

Note that an even more efficient encoding could be reached with a
“varbit”-style encoding as it is done within the Prometheus TSDB. However, I
assume the usual varint encoding as used by protobuf is a good tradeoff between
size on the wire and cost of encoding and decoding. Implementing the whole
encoding schema in protobuf also has the advantage that it is easy to create
encoders and decoders in any protobuf-supported language.

## Storage

The storage of the sparse histograms in a TSDB can utilize “triple-delta
encoding”. The first delta is already included in the exposition format, see
above. The usual double-delta encoding in the Prometheus TSDB can then be
applied on those first deltas, resulting in triple-delta encoding. If you run
the `scraper` program in the continuous scrape mode (TODO: to be implemented!),
it reports an estimate how many bits saving the triple-deltas will take, using
the existing bit-pattern schema in the Promtheus TSDB (TODO: explore other
patterns). The details in the storage need to be fleshed out, in particular how
to efficiently handle bucketing schema changes between scrapes, i.e. appearing
and disappearing buckets.
  
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
8191, all bucket counts are encodable in a varint-based delta encoding in less
than 200 bytes.

The protobuf encoding needs a bit of boilerplate around it (array sizes, field
numbers, etc.), wich results in a wire size of the whole `Histogram` proto
message of 317 bytes.

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
a 1-byte varint (between –64 and 63).

The wire size of the whole `Histogram` proto message is 244 bytes.

On my laptop, it took the exposer 550ms to read in the dataset, parse it, and
perform all the observations (300ns/observation). That's pretty decent for the
ad-hoc implementation in client_golang.

The `spamd.20190918` dataset fills 63 different buckets, 45 positive buckets,
17 negative buckets, and the “zero” bucket. Both the positive and negative
buckets have each 6 separate spans of consecutive buckets. The gaps are in the
range of small absolute values, which is caused by a ±0.1 delta corresponding
to a multi-bucket jump with small absolute values. The fairly irregular
distribution requires a 2-byte varint for about half of the deltas between
bucket counts.

The wire size of the whole `Histogram` proto message is 257 bytes.

For comparison: The `Histogram` proto message for the conventional fixed-bucket
Histogram in the [client_golang example
app](https://github.com/prometheus/client_golang/blob/master/examples/random/main.go)
has a size of 789 bytes for 11 buckets.
