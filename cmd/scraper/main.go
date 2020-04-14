package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"sort"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/prometheus/prom2json"

	dto "github.com/prometheus/client_model/go"
)

var (
	usage = fmt.Sprintf(`Usage: %s METRICS_URL`, os.Args[0])

	decode   = flag.Bool("decode", false, "Decode scraped histogram and dump to stdout.")
	interval = flag.Duration("scrape-interval", 0, "If 0, scrape once and exit. Otherwise, continuously scrape with this interval.")

	storages = map[string]*Storage{} // A Storage for each histogram, keyed by name + string representation of labels.
)

// Storage is a fake storage to collect some statistics about deltas of a single histogram.
// It is not (yet) concerned about bucket schema changes.
type Storage struct {
	Δp, Δn   map[int32]int64 // Last scraped deltas by index, for positive and negative buckets.
	ΔΔp, ΔΔn map[int32]int64 // Delta of the two most recently scraped deltas by index, for positive and negative buckets.
	ΔΔΔfreq  map[int64]uint  // Frequency of triple deltas by value for analysis.
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
							ReportDDDStats(s, os.Stdout)
						}
					}
				}
			}
		}
	}
}

func DumpAndTrack(h *dto.Histogram, s *Storage, dump io.Writer) {
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

	fmt.Fprintln(dump, "- Buckets:")
	signedDump(true)
	fmt.Fprintln(dump, " ", -h.GetSbZeroThreshold(), "≤ x ≤", h.GetSbZeroThreshold(), "→", h.GetSbZeroCount())
	signedDump(false)

}

func ReportDDDStats(s *Storage, o io.Writer) {
	var (
		vals     []int
		sum, cum uint
	)

	for val, count := range s.ΔΔΔfreq {
		vals = append(vals, int(val)) // Convert to int just for ease of sorting.
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
