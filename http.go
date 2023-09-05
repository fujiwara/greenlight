package greenlight

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	DefaultHTTPTimeout = 15 * time.Second
)

type HTTPCheckConfig struct {
	URL                string            `yaml:"url"`
	Method             string            `yaml:"method"`
	Headers            map[string]string `yaml:"headers"`
	Body               string            `yaml:"body"`
	ExpectCode         string            `yaml:"expect_code"`
	ExpectPattern      string            `yaml:"expect_pattern"`
	NoCheckCertificate bool              `yaml:"no_check_certificate"`
}

func NewHTTPChecker(cfg *CheckConfig) (*HTTPChecker, error) {
	p := &HTTPChecker{
		name:               cfg.Name,
		Timeout:            cfg.Timeout,
		NoCheckCertificate: cfg.HTTP.NoCheckCertificate,
		Headers:            cfg.HTTP.Headers,
		Body:               cfg.HTTP.Body,
	}
	var err error
	u, err := url.Parse(cfg.HTTP.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid url %s: %w", cfg.HTTP.URL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid url %s: scheme must be http or https", cfg.HTTP.URL)
	}
	p.URL = u.String()

	if pt := cfg.HTTP.ExpectPattern; pt != "" {
		p.ExpectPattern, err = regexp.Compile(pt)
		if err != nil {
			return nil, fmt.Errorf("invalid expect_pattern %s: %w", pt, err)
		}
	}
	// default
	if p.Method == "" {
		p.Method = http.MethodGet
	}
	if cfg.HTTP.ExpectCode == "" {
		p.ExpectCodeFunc = func(code int) bool {
			return code >= 200 && code < 400
		}
	} else {
		p.ExpectCodeFunc, err = newExpectCodeFunc(cfg.HTTP.ExpectCode)
		if err != nil {
			return nil, fmt.Errorf("invalid expect_code %s: %w", cfg.HTTP.ExpectCode, err)
		}
	}

	return p, nil
}

type HTTPChecker struct {
	URL                string
	Method             string
	Headers            map[string]string
	Body               string
	ExpectCodeFunc     func(code int) bool
	ExpectPattern      *regexp.Regexp
	Timeout            time.Duration
	NoCheckCertificate bool

	name string
}

func (p *HTTPChecker) Name() string {
	return p.name
}

func (p *HTTPChecker) Run(ctx context.Context) error {
	logger := newLoggerFromContext(ctx).With("name", p.name, "module", "httpchecker")

	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, p.Method, p.URL, strings.NewReader(p.Body))
	if err != nil {
		return err
	}
	for name, value := range p.Headers {
		req.Header.Set(name, value)
	}
	req.Header.Set("Connection", "close") // do not keep alive to health check.
	req.Header.Set("User-Agent", "greenlight")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: p.NoCheckCertificate},
	}
	client := &http.Client{Transport: tr}

	logger.Debug(fmt.Sprintf("http request %s %s", req.Method, req.URL))
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if p.ExpectCodeFunc != nil {
		if !p.ExpectCodeFunc(resp.StatusCode) {
			return fmt.Errorf("expect code not match: %d", resp.StatusCode)
		}
	}

	if p.ExpectPattern == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body failed: %w", err)
	}
	if p.ExpectPattern != nil {
		if !p.ExpectPattern.Match(body) {
			return fmt.Errorf("expect pattern not match: %s", p.ExpectPattern.String())
		}
	}
	return nil
}

// newExpectCodeFunc parses a string of comma separated HTTP status codes and
// returns a function that checks if the given code is in the list.
// e.g. "200,201,202-204,300-399"
func newExpectCodeFunc(codes string) (func(code int) bool, error) {
	ranges := strings.Split(codes, ",")
	var parsedRanges []struct{ lower, upper int }

	for _, r := range ranges {
		r = strings.TrimSpace(r) // Remove any leading and trailing whitespaces
		bounds := strings.Split(r, "-")
		for i := range bounds {
			bounds[i] = strings.TrimSpace(bounds[i]) // Trim spaces for each bound
		}
		if len(bounds) == 1 {
			// Single code
			singleCode, err := strconv.Atoi(bounds[0])
			if err != nil {
				return nil, errors.New("invalid code: " + bounds[0])
			}
			parsedRanges = append(parsedRanges, struct{ lower, upper int }{singleCode, singleCode})
		} else if len(bounds) == 2 {
			// Range of codes
			lower, err1 := strconv.Atoi(bounds[0])
			upper, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil {
				return nil, errors.New("invalid range: " + r)
			}
			parsedRanges = append(parsedRanges, struct{ lower, upper int }{lower, upper})
		} else {
			return nil, errors.New("invalid format: " + r)
		}
	}

	return func(code int) bool {
		for _, r := range parsedRanges {
			if r.lower <= code && code <= r.upper {
				return true
			}
		}
		return false
	}, nil
}
