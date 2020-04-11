package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"

	"github.com/gogo/protobuf/proto"
	"github.com/prometheus/prom2json"

	dto "github.com/prometheus/client_model/go"
)

var usage = fmt.Sprintf(`Usage: %s [METRICS_PATH | METRICS_URL ]`, os.Args[0])

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
				}
			}
		}
	}
}
