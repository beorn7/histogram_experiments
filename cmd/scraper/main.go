package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/url"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/prometheus/prom2json"

	dto "github.com/prometheus/client_model/go"
)

var (
	usage = fmt.Sprintf(`Usage: %s [METRICS_PATH | METRICS_URL ]`, os.Args[0])

	decode = flag.Bool("decode", false, "Decode scraped histogram and dump to stdout.")
)

func main() {
	flag.Parse()

	var input io.Reader
	var err error
	arg := flag.Arg(0)
	flag.NArg()

	if flag.NArg() > 1 {
		log.Fatalf("Too many arguments.\n%s", usage)
	}

	if arg == "" {
		// Use stdin on empty argument.
		input = os.Stdin
	} else if url, urlErr := url.Parse(arg); urlErr != nil || url.Scheme == "" {
		// `url, err := url.Parse("/some/path.txt")` results in: `err == nil && url.Scheme == ""`
		// Open file since arg appears not to be a valid URL (parsing error occurred or the scheme is missing).
		if input, err = os.Open(arg); err != nil {
			log.Fatal("error opening file:", err)
		}
	}
	mfChan := make(chan *dto.MetricFamily, 1024)

	// Missing input means we are reading from an URL.
	if input != nil {
		go func() {
			if err := prom2json.ParseReader(input, mfChan); err != nil {
				log.Fatal("error reading metrics:", err)
			}
		}()
	} else {
		go func() {
			err := prom2json.FetchMetricFamilies(arg, mfChan, nil)
			if err != nil {
				log.Fatalln(err)
			}
		}()
	}

	for mf := range mfChan {
		if mf.GetType() == dto.MetricType_HISTOGRAM {
			for _, m := range mf.GetMetric() {
				h := m.GetHistogram()
				if h.GetSbResolution() > 0 {
					fmt.Println("### Found sparse histogram:", mf.GetName(), m.GetLabel())
					buf, err := proto.Marshal(h)
					if err != nil {
						panic(err)
					}
					fmt.Println("- Bytes in Histogram message on the wire:", len(buf))
					if *decode {
						Dump(h, os.Stdout)
					}
				}
			}
		}
	}
}

func Dump(h *dto.Histogram, o io.Writer) {
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
			curCount int64
		)
		buckets := h.GetSbPositive()
		if negative {
			buckets = h.GetSbNegative()
		}
		for _, span := range buckets.GetSpan() {
			curIdx += span.GetOffset()
			if bound(curIdx-1) > threshold {
				lines = append(lines, separator)
			}
			for nextIdx := curIdx + int32(span.GetLength()); curIdx < nextIdx; curIdx++ {
				curCount += buckets.GetDelta()[deltaPos]
				deltaPos++
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
				fmt.Fprint(o, lines[i])
			}
		} else {
			for _, line := range lines {
				fmt.Fprint(o, line)
			}
		}
	}

	fmt.Fprintln(o, "- Buckets:")
	signedDump(true)
	fmt.Fprintln(o, " ", -h.GetSbZeroThreshold(), "≤ x ≤", h.GetSbZeroThreshold(), "→", h.GetSbZeroCount())
	signedDump(false)
}
