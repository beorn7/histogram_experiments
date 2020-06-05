package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/bits"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/prometheus/prom2json"

	dto "github.com/prometheus/client_model/go"
)

var (
	usage = fmt.Sprintf(`Usage: %s METRICS_URL`, os.Args[0])

	decode     = flag.Bool("decode", false, "Decode scraped histogram and dump to stdout.")
	interval   = flag.Duration("scrape-interval", 0, "If 0, scrape once and exit. Otherwise, continuously scrape with this interval.")
	bitBuckets bitBucketsFlag

	storages = map[string]*Storage{} // A Storage for each histogram, keyed by name + string representation of labels.
)

func init() {
	flag.Var(&bitBuckets, "bit-buckets", "Comma-separated list of bit bucket boundaries. (Gorilla uses '7,9,12,32' for timestamps, Prometheus 2 '14,17,20,64' for timestamps, Prometheus 1 '6,17,23' for timestamps and '6,13,20,33' for integer values.) Leave empty to print the frequency of every occurring value instead of a storage analysis. Use '0' to trigger a search for the best bucketing with one to four buckets.")
}

type bitBucketsFlag []int // Use int to use sort.Int.

func (bbf *bitBucketsFlag) String() string {
	return fmt.Sprint(*bbf)
}

func (bbf *bitBucketsFlag) Set(value string) error {
	if len(*bbf) > 0 {
		return errors.New("interval flag already set")
	}
	for _, bt := range strings.Split(value, ",") {
		b, err := strconv.ParseUint(bt, 0, 8)
		if err != nil {
			return err
		}
		*bbf = append(*bbf, int(b))
	}
	sort.Ints(*bbf)
	return nil
}

// Storage is a fake storage to collect some statistics about deltas of a single histogram.
// It is not (yet) concerned about bucket schema changes.
type Storage struct {
	Δp, Δn   map[int32]int64 // Last scraped deltas by index, for positive and negative buckets.
	ΔΔp, ΔΔn map[int32]int64 // Delta of the two most recently scraped deltas by index, for positive and negative buckets.
	ΔΔΔfreq  map[int64]uint  // Frequency of triple deltas by value for analysis.
	n        uint            // Total number of scrapes.
}

func NewStorage() *Storage {
	return &Storage{
		Δp:      map[int32]int64{},
		Δn:      map[int32]int64{},
		ΔΔp:     map[int32]int64{},
		ΔΔn:     map[int32]int64{},
		ΔΔΔfreq: map[int64]uint{},
	}
}

func main() {
	flag.Parse()

	arg := flag.Arg(0)
	flag.NArg()

	if flag.NArg() != 1 {
		log.Fatalf("Need exactly one argument.\n%s", usage)
	}
	if *interval == 0 {
		Scrape(arg)
		return
	}
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		Scrape(arg)
		<-ticker.C
	}
}

func Scrape(url string) {
	mfChan := make(chan *dto.MetricFamily, 1024)
	go func() {
		err := prom2json.FetchMetricFamilies(url, mfChan, nil)
		if err != nil {
			log.Fatalln(err)
		}
	}()

	for mf := range mfChan {
		if mf.GetType() == dto.MetricType_HISTOGRAM {
			for _, m := range mf.GetMetric() {
				h := m.GetHistogram()
				if h.GetSbResolution() > 0 {
					key := fmt.Sprint(mf.GetName(), m.GetLabel())
					fmt.Println("### Found sparse histogram:", key)
					buf, err := proto.Marshal(h)
					if err != nil {
						panic(err)
					}
					fmt.Println("- Bytes in Histogram message on the wire:", len(buf))
					if *decode || *interval != 0 {
						dump := ioutil.Discard
						if *decode {
							dump = os.Stdout
						}
						s := storages[key]
						if s == nil {
							s = NewStorage()
							storages[key] = s
						}
						DumpAndTrack(h, s, dump)
						if *interval != 0 {
							if len(bitBuckets) == 0 {
								ReportΔΔΔStats(s, os.Stdout)
							} else if bitBuckets[0] == 0 {
								BruteForceBitBucketSearch(s, os.Stdout)
							} else {
								ReportBitBucketStats(s, bitBuckets, os.Stdout)
							}
						}
					}
				}
			}
		}
	}
}

func DumpAndTrack(h *dto.Histogram, s *Storage, dump io.Writer) {
	s.n++
	separator := "  ----------------------------------------------------------------------\n"
	resolution := int32(h.GetSbResolution())
	threshold := h.GetSbZeroThreshold()
	bound := func(i int32) float64 {
		var result float64
		if i%resolution == 0 {
			result = float64(math.Pow10(int(i / resolution)))
		} else {
			result = math.Pow(10, float64(i)/float64(resolution))
		}
		if result < threshold {
			return threshold
		}
		return result
	}
	signedDump := func(negative bool) {
		// This function assumes well behaved spans:
		// - Only start a new one if really needed (i.e. an actual gap).
		// - No zero length spans.
		// - Total length of spans == length of deltas.
		// A robust implementation should handle degenerate cases gracefully.
		var (
			lines    []string
			curIdx   int32
			deltaPos int
			curCount = int64(h.GetSbZeroCount())
		)
		buckets := h.GetSbPositive()
		Δold, ΔΔold := s.Δp, s.ΔΔp
		if negative {
			buckets = h.GetSbNegative()
			Δold, ΔΔold = s.Δn, s.ΔΔn
		}
		Δnew, ΔΔnew := map[int32]int64{}, map[int32]int64{}

		for _, span := range buckets.GetSpan() {
			curIdx += span.GetOffset()
			if bound(curIdx-1) > threshold {
				lines = append(lines, separator)
			}
			for nextIdx := curIdx + int32(span.GetLength()); curIdx < nextIdx; curIdx++ {
				Δ := buckets.GetDelta()[deltaPos]
				deltaPos++
				Δnew[curIdx] = Δ
				ΔΔ := Δ - Δold[curIdx]
				ΔΔnew[curIdx] = ΔΔ
				s.ΔΔΔfreq[ΔΔ-ΔΔold[curIdx]]++

				curCount += Δ
				if negative {
					lines = append(lines, fmt.Sprintln(
						" ", -bound(curIdx), "≤ x <", -bound(curIdx-1), "→", curCount,
					))
				} else {
					lines = append(lines, fmt.Sprintln(
						" ", bound(curIdx-1), "< x ≤", bound(curIdx), "→", curCount,
					))
				}
			}
		}

		if negative {
			for i := len(lines) - 1; i >= 0; i-- {
				fmt.Fprint(dump, lines[i])
			}
			s.Δn, s.ΔΔn = Δnew, ΔΔnew
		} else {
			for _, line := range lines {
				fmt.Fprint(dump, line)
			}
			s.Δp, s.ΔΔp = Δnew, ΔΔnew
		}
	}

	nBuckets := len(h.GetSbNegative().GetDelta()) + 1 + len(h.GetSbPositive().GetDelta())
	nSpans := len(h.GetSbNegative().GetSpan()) + len(h.GetSbPositive().GetSpan())
	fmt.Fprintln(dump, "-", nBuckets, "buckets /", nSpans, "spans:")
	signedDump(true)
	fmt.Fprintln(dump, " ", -h.GetSbZeroThreshold(), "≤ x ≤", h.GetSbZeroThreshold(), "→", h.GetSbZeroCount())
	signedDump(false)

}

func ReportΔΔΔStats(s *Storage, o io.Writer) {
	var (
		vals     []int
		sum, cum uint
	)

	for val, count := range s.ΔΔΔfreq {
		vals = append(vals, int(val)) // Convert to int just for ease of sorting with sort.Ints.
		sum += count
	}
	sort.Ints(vals)

	fmt.Fprintln(o, "- ΔΔΔ frequency:")
	for _, val := range vals {
		count := s.ΔΔΔfreq[int64(val)]
		cum += count
		fmt.Fprintf(o, "  %d → %d (%.2f%%)\n", val, count, float64(cum)/float64(sum)*100)
	}
}

// ReportBitBucketStats returns the total number of bits used.
func ReportBitBucketStats(s *Storage, bitBuckets []int, o io.Writer) uint {
	bs := make([]uint, len(bitBuckets)+1)
	limits := make([]int64, len(bitBuckets))
	for i, bb := range bitBuckets {
		limits[i] = 1 << (bb - 1)
	}

	var total uint
Outer:
	for val, count := range s.ΔΔΔfreq {
		total += count
		if val == 0 {
			bs[0] += count
			continue
		}
		for i, limit := range limits {
			if val < limit && val >= -limit {
				bs[i+1] += count
				continue Outer
			}
		}
		log.Fatalln("ΔΔΔ value", val, "doesn't fit into largest bit bucket.")
	}

	fmt.Fprintf(o, "- Bit bucket frequency (%d buckets incl. zero bucket):\n", len(bitBuckets)+1)
	for i, b := range bs {
		bits := 0
		if i != 0 {
			bits = bitBuckets[i-1]
		}
		fmt.Fprintf(o, "  %d bits → %d (%.2f%%)\n", bits, b, float64(b)/float64(total)*100)
	}

	totalBits := bs[0] // Each zero entry takes 1bit.
	for i, bb := range bitBuckets {
		bitsPerValue := i + 2 + bb
		if i == len(bitBuckets)-1 {
			bitsPerValue-- // Last bucket has one marker bit less.
		}
		totalBits += uint(bitsPerValue) * bs[i+1]
	}
	fmt.Fprintf(o, "  TOTAL storage size for ΔΔΔ values: %d bytes (%.1f bytes per scrape)\n", totalBits/8, float64(totalBits)/8/float64(s.n))
	return totalBits
}

func BruteForceBitBucketSearch(s *Storage, o io.Writer) {
	var (
		maxVal int64 = 1
		minVal int64 = -1
	)
	for val := range s.ΔΔΔfreq {
		if val < minVal {
			minVal = val
		} else if val > maxVal {
			maxVal = val
		}
	}
	largestBucket := bits.Len64(uint64(maxVal)) + 1
	largestBucketForMin := bits.Len64(uint64(-minVal-1)) + 1
	if largestBucketForMin > largestBucket {
		largestBucket = largestBucketForMin
	}
	ReportBitBucketStats(s, []int{largestBucket}, o)

	for numExtraBuckets := 1; numExtraBuckets < 4; numExtraBuckets++ {
		BruteForceBitBucketSearchN(s, largestBucket, numExtraBuckets, o)
	}
}

func BruteForceBitBucketSearchN(s *Storage, largestBucket int, numExtraBuckets int, o io.Writer) {
	if numExtraBuckets >= largestBucket {
		return // Cannot even fit that many buckets.
	}

	var (
		currentBuckets, bestBuckets []int
		bestBits                    uint = math.MaxUint32
	)
	for i := 1; i <= numExtraBuckets; i++ {
		currentBuckets = append(currentBuckets, i)
	}
	currentBuckets = append(currentBuckets, largestBucket)

	for next := true; next; next = IncrementBuckets(currentBuckets, len(currentBuckets)-2) {
		bits := ReportBitBucketStats(s, currentBuckets, ioutil.Discard)
		if bits < bestBits {
			bestBits = bits
			bestBuckets = append(bestBuckets[:0], currentBuckets...)
		}
	}
	ReportBitBucketStats(s, bestBuckets, o)
}

func IncrementBuckets(b []int, p int) bool {
	if b[p+1]-b[p] > 1 {
		b[p]++
		return true
	}
	if p == 0 {
		return false
	}
	if b[p]-b[p-1] > 2 {
		b[p] = b[p-1] + 2
		for q := p + 1; q < len(b)-1; q++ {
			b[q] = b[q-1] + 1
		}
	}
	return IncrementBuckets(b, p-1)
}
