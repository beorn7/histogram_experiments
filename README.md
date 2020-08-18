# Histogram Experiments

To play with new approaches to represent histograms in Prometheus, this
repository provides two tools:

1. `exposer`: Reads in observations from a dataset and exposes them in a
   histogram.
2. `scraper`: Scrapes a target with histograms (usually `exposer`) and prints
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
  family). The observed values here are rounded to one decimal place, but they
  can be negative. The distribution is fairly irregular. This dataset is
  intended to test how well the histogram works with an atypical distribution,
  not related to the usual request latency measurement.
  
## The basic idea

The basic idea is to have an “infinite buckets” histogram with a regular
logarithmic (or logarithmic-linear) bucketing schema and efficient handling of
sparseness (i.e. empty buckets don't take any resources). This idea has been
used in other implementations for quite some time, e.g. HDR Histogram,
circlllhist, and more recently DDSketch. For a detailed comparison, see [a
recent paper by Heinrich Hartmann and Theo
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
  far, a value between 20 and 100 seems to be a reasonable range. Note that
  keeping this number the same over time allows merging of histograms, while
  changing it disrupts that. The buckets are then spaced logarithmically,
  resulting in “weird” bucket boundaries, but every power of 10 will also be a
  bucket boundary (10ms, 100ms, 1s, 10s, …). (The circlllhist idea of spacing
  buckets linearly within a power of 10 to have “round” numbers could easily be
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
collect observations for a fixed time frame (like one minute) and then sent it
off for good to the metrics collection system, after which all bucket counters
can be reset to zero, reestablishing sparseness of the histogram. With
Prometheus, monitored targets accumulate counts essentially forever and accept
“stateless” scrapes from any number of scrapers. The histogram will become less
and less sparse because of that, with more and more of the infinite number of
buckets being used. The assumption is, however, that with real-world
observations, the number of used buckets will quickly approach a not too large
number, so that resetting a frequently updated histogram every minute is
actually not so much different from never resetting it. The datasets provided
in this repo are used to test that assumption, see observations below.

Having said that, a very occasional (i.e. rare compared to the scrape interval)
reset of a histogram is relatively harmless. For example, with a typical scrape
interval of 15s, resetting all histograms every hour would only lose 0.2% of
counts. That's particularly helpful if there was a short time span during which
“exotic” buckets were populated, e.g. during degraded performance.

Another approach to limiting the bucket count would be to dynamically widen the
“zero bucket”, which is essentially what DDSketch is doing.

## Effect of resolution on the error of quantile estimation

The experiments below have been performed for a resolution of 20 buckets per
power of ten and 100 buckets per power of ten, assuming that most use cases
will require a resolution in this range.

If φ-quantiles are estimated by choosing the [harmonic
mean](https://en.wikipedia.org/wiki/Harmonic_mean) of the boundaries of the
bucket they fall into, the relative error is guaranteed to be below 5.8% for a
resolution of 20. For a resolution of 100, the maximum relative error is
1.2%. (Exception: Quantiles that fall into the “zero bucket”. Their absolute
error is still bounded by the width of the “zero bucket”, but the relative
error close to zero is inevitably approaching infinity.)

For log-linear buckets as in circlllhist (not implemented here, just for
reference), the maximum relative error depends on which bucket within a power
of 10 the quantile falls into. circlllhist uses 90 linear buckets per power of
10, so that the boundaries coincide with round decimal numbers (e.g. 10ms,
11ms, 12ms, … , 99ms, 100ms, 110ms, 120ms, …). In the 1st bucket, the maximum
error is 4.8%. In the last (90th) bucket, it is 0.5%.

In the context of the quantile estimation error, there are a number of things
to keep in mind:
- The current implementation of `histogram_quantile` in Prometheus doesn't use
  the harmonic mean but interpolates the quantile, assuming a uniform
  distribution of observations within the bucket. This is expected to give on
  average better results for most real-world distributions, but more than
  doubles the possible worst-case relative error (e.g. 12.2% in case of a
  resolution of 20, if the same kind of linear interpolation is used for sparse
  histograms).
- The quantile estimation answers a question like: “What is the 99th percentile
  latency?” However, often a slightly different question is more relevant:
  “What is the percentage of requests served faster than 150ms?” The latter
  question can be answered precisely with a bucket boundary at 150ms, which is
  an important advantage of the existing Prometheus Histograms, provided you
  actually have set a bucket boundary at that value. It is also the main reason
  behind circlllhist, which assumes that that question is usually asked with
  round decimal numbers and therefore trades off a relatively high worst-case
  error against the ability to answer that question precisely.
- The estimation error discussed here so far is in the dimension of the sampled
  values, e.g. “the 90th percentile latency is between 110ms and
  120ms”. However, the error could also be measured in the φ dimension,
  e.g. “115ms is between the 89th and 91st percentile latency”. The latter is
  commonly used in papers about digest-like approaches, which interestingly
  touches a problem of digest-like approaches for Prometheus use-cases: A
  quantile estimation error like φ±0.01 looks great on paper, but if
  calculating the 99th percentile, it boils down to a φ between 0.98 and
  1.00. For the 99.9th percentile, it's between 0.989 and 1.009. In other words,
  digest-like approaches usually struggle a lot if dealing with the common use
  case of long-tail latencies, up to a point where the estimations are
  completely useless. The bucketed approaches, on the other hand, are well
  suited for distributions with a long tail.

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
- Each `SparseBuckets` message contains the following:
  - The delta-encoded counts in the buckets, i.e. each value is the difference
    to the previous bucket (or to zero in case of the first bucket), assuming
    that those differences are smaller than absolute numbers in most real-world
    use cases. The type is `sint64`, which uses varint encoding in a way that's
    efficient for negative numbers, too. (The deltas are expected to be
    negative about as often as positive.) In usual benign use cases, most of
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
assume the usual varint encoding as used by protobuf is a good trade-off between
size on the wire and cost of encoding and decoding. Implementing the whole
encoding schema in protobuf also has the advantage that it is easy to create
encoders and decoders in any protobuf-supported language.

## Storage

Fundamentally, a single “sample value” of a sparse histogram is considered to
be the whole set of buckets, i.e. only one index entry per histogram, not per
bucket, and ingestion of all buckets in one go, without multiplexing to one
sample stream per bucket.

At the start of a chunk in the Prometheus TSDB, a sparse histogram will be
saved as bucket deltas, in the same way as represented in the exposition
format.

There are two options how to proceed from there for the remaining samples in
the chunk: The bucket deltas could be encoded using the same double-delta
encoding as already used for timestamps (which also was used by Prometheus 1
for sample values that are integer numbers). As that double-delta encodes the
bucket deltas, I call it “triple-delta encoding”. The other option is to
reconstruct the absolute bucket counts from the bucket deltas in the exposition
format and then double-delta encode those absolute counts, resulting in
“normal” double-delta encoding, but still with the difference that all bucket
counts of a histogram are encoded in one sample (rather than multiplexed into
one sample stream per bucket).

Both options are explored below to find out which one encodes more
efficiently. The double-delta encoding is slightly more expensive during
ingestion (because the absolute bucket counts need to be re-constructed first),
but I doubt it will be prohibitive. The double-delta encoding might appear more
straight forward and possibly even easier to decode, but in most practical
application, I expect the decoding to scan through most of the encoded data
anyway. A few more additions of integers during decoding should hardly make a
dent.

With either option, the resulting triple or double deltas are encoded with a
variable bitwidth schema as used already in the Prometheus TSDB.

If you run the `scraper` program in the continuous scrape mode, it reports an
estimate how many bytes saving the triple deltas or double deltas (the latter
to be selected with the `--store-bucket-count` flag) will take, using
configurable bit-length patterns, called _bit-buckets_ (but those buckets are
unrelated to the buckets in the histogram). There is also a mode to
automatically find the optimal bit-bucketing for the given data. (Incidentally,
as a byproduct of my research, I found that Prometheus 2.x uses bit-buckets
that are almost certainly sub-optimal.)

The details in the storage need to be fleshed out, in particular how to
efficiently handle bucketing schema changes between scrapes, i.e. appearing and
disappearing buckets. The required storage bytes mentioned above are only for
the triple or double deltas, assuming that storing the bucketing schema will
take a much smaller amount of space. A rough idea would be that every
Prometheus TSDB chunk (which currently holds at most 120 samples, but that
needs to be revisited anyway, not only for histograms) saves one bucketing
schema that works for all samples in the chunk. As an additional optimization
during “post-processing” a chunk, buckets that never change throughout the
chunk could be taken out of the schema, with their constant value saved as a
single number. In that way, “exotic” buckets that rarely get updated (e.g. high
latency buckets that only got updates during a short outage and then never
again) will take even less space than the single bit per scrape. Another option
would be to perform run-length encoding on streaks of consecutive zeros.
  
## Observations

### Exposition size after ingesting the whole dataset

#### `cortex-querier.1h.20200309`

With the very broad dataset and a resolution (per power of 10) of 20, the
resulting sparse histogram has 112 buckets, from index –80 (approx. 89.1µs < x
≤ 100µs) to index 31 (approx. 31.6s < x ≤ 35.5s). Every bucket in this range
has at least one entry. The largest count difference between consecutive
buckets is 9240 (between index –54 and –53). The most populated bucket is index
–48 (3.55ms < x ≤ 3.98ms, 43335 observations).

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
numbers, etc.), which results in a wire size of the whole `Histogram` proto
message of 317 bytes.

With a resolution of 100, the histogram uses 535 buckets in 15 spans and needs
1294 bytes on the wire.

#### `cortex-ingester.2m.20200326` 

This dataset has almost 3x the number of observations, but uses only 78 buckets
at a resolution of 20, with a quite sharp maximum at index –78 (approx. 112µs <
x ≤ 126µs, 280941 observations). The buckets range from index –91 to –13, with
all buckets in between except the one at index –15 having at least one
observations. The largest count difference between consecutive buckets is 70867
(between index –81 and –80).

Again, encoding the schema of used buckets is efficient as spans (two spans in
this case: 76 consecutive buckets from –91, 2 consecutive buckets from
–14). The largest delta between buckets needs 3 bytes as a varint, but even
with the high observation count here, most deltas (45 out of 78) would fit into
a 1-byte varint (between –64 and 63).

The wire size of the whole `Histogram` proto message is 244 bytes.

On my laptop, it took the exposer 550ms to read in the dataset, parse it, and
perform all the observations (300ns/observation). That's pretty decent for the
ad-hoc implementation in client_golang.

With a resolution of 100, the histogram uses 355 buckets in 12 spans and needs
898 bytes on the wire.

#### `spamd.20190918`

This dataset fills 63 different buckets, 45 positive buckets, 17 negative
buckets, and the “zero” bucket. Both the positive and negative buckets have
each 6 separate spans of consecutive buckets. The gaps are in the range of
small absolute values, which is caused by a ±0.1 delta corresponding to a
multi-bucket jump with small absolute values. The fairly irregular distribution
requires a 2-byte varint for about half of the deltas between bucket counts.

The wire size of the whole `Histogram` proto message is 257 bytes.

With a resolution of 100, the histogram uses 175 buckets in 61 spans and needs
788 bytes on the wire.

#### Comparison to the current exposition format

The `Histogram` proto message for the conventional fixed-bucket Histogram in
the [client_golang example
app](https://github.com/prometheus/client_golang/blob/master/examples/random/main.go)
has a size of 789 bytes for 11 buckets.

### Simulated scrapes and estimation of storage size

To get an idea how much storage space will be needed for the sparse histogram,
I have run simulated scrapes with the `cortex-querier.1h.20200309` and
`cortex-ingester.2m.20200326` datasets and calculated the space that the triple
or double delta values would take, including the initial bucket delta values,
assuming that the whole scrape result would go into a single chunk.

The simulated scrape interval was 15s, but because of the very high frequency
but at the same time short duration of the `cortex-ingester.2m.20200326`
dataset, the exposition for the latter was slowed-down by a factor of 10,
resulting in 20m of data collection, 80 scrapes in total. The
`cortex-querier.1h.20200309` dataset was left at its original speed, resulting
in 240 scrapes.

Both datasets are aggregated from many instances and are not partitioned in any
way. Thus, the simulation is most relevant to an histogram resulting from an
aggregation rule, resulting in a relatively high entropy and frequent bucket
count increments. Histograms scraped from real individual instances, possibly
even partitioned by status code and other dimensions, would have less frequent
bucket count increments and less entropy and therefore a better compression
ratio, but there would be many more of them. The goal of the simulation was to
try out the “worst case” and see if we can handle it.

Note that the `spamd.20190918` was collected over a long time so that a
realistic simulated scrape would be dominated by delta values of 0. Therefore,
it wasn't considered for this analysis.

The detailed results are in a [separate
file](simulated_scrape_raw_results.md). The following is just a summary of the
most important conclusions:

- Triple vs. delta encoding: The triple encoding is about 2.5% more efficient
  for a resolution of 20. With the high resolution of 100, it is about the same
  for the ingester dataset and about 5% worse for the querier dataset. Thus,
  there is no clear winner, but the triple delta encoding seems to generally fare
  better. Since the triple delta encoding is also cheaper during ingestion (and
  all data is ingested once but most likely read less than once on average),
  I'm inclined to pick triple delta encoding for now.
- Bucketing for the varbit encoding: The simulations have calculated ideal bit
  bucketing, which – unsurprisingly – depends heavily on the nature of the
  dataset. We might explore dynamic bitbucketing in the future (including for
  regular Prometheus sample and timestamp encoding!), but for the time being,
  we should probably pick a “one size fits it all” bitbucketing. I picked a
  3/6/9/12/64 bucketing as the one being in the middle of the calculated ideal
  bucketing schema (with the added 64 bit bucket to be used extremely rarely
  for outliers).
- 30% to 50% of delta values are zero. No analysis has been performed on how
  often there are streaks of consecutive zeros. Those would benefit from a
  run-length encoding. However, since the zero-deltas only take 1bit each, the
  potential gain is much lower than it naively appears. The broader bit buckets
  take much more space. With the marker bits included, each entry in the 3bit
  bucket takes 5bit, in the 6bit bucket 9bit, and in the 9bit bucket 13bit. For
  example, a typical distribution of 40% in the 0bit bucket, 30% in the 3bit
  bucket, 20% in the 6bit bucket, and 10% in the 9bit bucket results in only 8%
  of the total storage space being used by zero deltas. Therefore, only in
  cases more dominated by rarely changing buckets would a run-length encoding
  or a special handling of constant buckets make a significant impact. We
  should still keep it in mind because those cases might be common in practice.
- At low resolution, a single histogram takes about 50 bytes for the ingester
  dataset and about 70 bytes for the querier dataset. At high resolution, the
  respective numbers are 180 bytes and 250 bytes. The space needed per bucket
  in each scrape is between 0.45 bytes and 0.65 bytes, depending on resolution
  and dataset. This is broadly a factor of three better than the per-sample
  space typically seen in the Prometheus TSDB. However, a sparse histogram has
  hundreds of buckets rather then tens in a conventional histogram. Therefore,
  the much higher resolution and “infinite” range of a sparse histogram _will_
  cost more storage cost. The assumption is that the decreased indexing cost
  will compensate enough so that sparse histograms are still feasible.


