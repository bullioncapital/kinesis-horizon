// alb-replay replays the successful GET requests found in an AWS ALB log file
// see https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html
package main

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	albLogEntryCount               = 29
	albTargetStatusCodeRecordIndex = 9
	albRequestIndex                = 12
)

type NumberedURL struct {
	Number int
	URL    string
}

type ALBLogEntryReader struct {
	csvReader *csv.Reader
	config    ALBLogEntryReaderConfig
}

type ALBLogEntryReaderConfig struct {
	pathRegexp       *regexp.Regexp
	statusCodeRegexp *regexp.Regexp
}

func newALBLogEntryReader(input io.Reader, config ALBLogEntryReaderConfig) *ALBLogEntryReader {
	reader := csv.NewReader(input)
	reader.Comma = ' '
	reader.FieldsPerRecord = albLogEntryCount
	reader.ReuseRecord = true
	return &ALBLogEntryReader{
		csvReader: reader,
		config:    config,
	}
}

func (r *ALBLogEntryReader) GetRequestURI() (string, error) {
	records, err := r.csvReader.Read()
	if err != nil {
		return "", err
	}

	statusCodeStr := records[albTargetStatusCodeRecordIndex]
	// discard requests with unknown status code
	if statusCodeStr == "-" {
		return "", nil
	}

	if !r.config.statusCodeRegexp.MatchString(statusCodeStr) {
		// discard url
		return "", nil
	}

	reqStr := records[albRequestIndex]
	reqFields := strings.Split(reqStr, " ")
	if len(reqFields) != 3 {
		return "", fmt.Errorf("error parsing request %q: 3 fields exepcted, found %d", reqStr, len(reqFields))
	}
	method := reqFields[0]

	// discard non-get requests
	if method != http.MethodGet {
		return "", nil
	}

	urlStr := reqFields[1]
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("error parsing url %q: %v", urlStr, err)
	}

	if r.config.pathRegexp != nil {
		if !r.config.pathRegexp.MatchString(urlStr) {
			// discard url
			return "", nil
		}
	}

	return parsed.RequestURI(), nil
}

func parseURLs(ctx context.Context, startFrom int, baseURL string, logReader *ALBLogEntryReader, urlChan chan NumberedURL) {
	counter := 0
	for {
		uri, err := logReader.GetRequestURI()
		if err != nil {
			if err == io.EOF {
				// we are done
				return
			}
			log.Fatal(err.Error())
		}
		if uri == "" {
			// no usable URL found in the current log line
			continue
		}
		counter++
		if counter < startFrom {
			// we haven't yet reached the expected start point
			continue
		}
		url := NumberedURL{
			Number: counter,
			URL:    baseURL + uri,
		}
		select {
		case <-ctx.Done():
			return
		case urlChan <- url:
		}
	}
}

func queryURLs(ctx context.Context, timeout time.Duration, urlChan chan NumberedURL, quiet bool) {
	client := http.Client{
		Timeout: timeout,
	}
	for numURL := range urlChan {
		if ctx.Err() != nil {
			return
		}

		req, err := http.NewRequest(http.MethodGet, numURL.URL, nil)
		if err != nil {
			log.Printf("(%d) unexpected error creating request: %v", numURL.Number, err)
			continue
		}
		req = req.WithContext(ctx)
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			// we don't want to print cancel errors due to a signal interrupt
			if errors.Unwrap(err) != context.Canceled {
				log.Printf("(%d) unexpected request error: %v %q", numURL.Number, errors.Unwrap(err), numURL.URL)
			}
			continue
		}
		resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			log.Printf("(%d) unexpected status code: %d %q", numURL.Number, resp.StatusCode, numURL.URL)
			continue
		}
		if !quiet {
			log.Printf("(%d) %s %s", numURL.Number, time.Since(start), numURL.URL)
		}
	}
}

func main() {
	workers := flag.Int("workers", 1, "How many parallel workers to use")
	startFromURLNum := flag.Int("start-from", 1, "What URL number to start from")
	pathRegexp := flag.String("path-filter", "", "Regular expression with which to filter in requests based on their paths")
	statusCodeRegexp := flag.String("status-code-filter", "^2[0-9][0-9]$", "Regular expression with which to filter in request based on their status codes")
	timeout := flag.Duration("timeout", time.Second*5, "HTTP request timeout")
	quiet := flag.Bool("quiet", false, "Only log failed requests")
	flag.Parse()
	if *workers < 1 {
		log.Fatal("--workers parameter must be > 0")
	}
	if *startFromURLNum < 1 {
		log.Fatal("--start-from must be > 0")
	}
	var pathRE *regexp.Regexp
	if *pathRegexp != "" {
		var err error
		pathRE, err = regexp.Compile(*pathRegexp)
		if err != nil {
			log.Fatalf("error parsing --path-filter %q: %v", *pathRegexp, err)
		}
	}
	var statusCodeRE *regexp.Regexp
	if *statusCodeRegexp != "" {
		var err error
		statusCodeRE, err = regexp.Compile(*statusCodeRegexp)
		if err != nil {
			log.Fatalf("error parsing --status-code-filter parameter %q: %v", *statusCodeRegexp, err)
		}
	}
	if flag.NArg() != 2 {
		log.Fatalf("usage: %s <aws_log_file[.gz]> <target_host_base_url>", os.Args[0])
	}

	var reader io.ReadCloser
	filePath := flag.Args()[0]
	reader, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("error opening file %q: %v", filePath, err)
	}
	defer reader.Close()
	if filepath.Ext(filePath) == ".gz" {
		var err error
		reader, err = gzip.NewReader(reader)
		if err != nil {
			log.Fatalf("error opening file %q: %v", filePath, err)
		}
		defer reader.Close()
	}

	baseURL := flag.Args()[1]
	logReaderConfig := ALBLogEntryReaderConfig{
		pathRegexp:       pathRE,
		statusCodeRegexp: statusCodeRE,
	}
	logReader := newALBLogEntryReader(reader, logReaderConfig)
	urlChan := make(chan NumberedURL, *workers)
	var wg sync.WaitGroup

	// setup interrupt cleanup code
	ctx, stopped := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopped()

	// spawn url consumers
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			queryURLs(ctx, *timeout, urlChan, *quiet)
			wg.Done()
		}()
	}

	parseURLs(ctx, *startFromURLNum, baseURL, logReader, urlChan)
	// signal the consumers there won't be more urls
	close(urlChan)
	// wait for to consumers to be done
	wg.Wait()
}
