// gryvia-flight fetches a job's Flight Recorder report from the Gryvia API gateway.
//
// It talks to the gateway (GET /api/flight/jobs/{job}), not to collectors: the gateway holds the
// collector token, signs the per-node requests, applies tenant scoping and reports coverage. The
// credential used here is the gateway's API key or a session/OIDC token, never the collector token.
//
// Exit codes: 0 report fetched; 1 request or response failure; 2 usage error; 3 the report is
// partial or truncated and --require-complete was given.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	maxReport    = 4 << 20
	maxErrorBody = 4 << 10
	tokenEnv     = "GRYVIA_API_TOKEN"
)

var dnsLabel = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// report holds the fields of the gateway response this tool interprets. Everything else is passed
// through untouched in JSON output.
type report struct {
	Namespace string `json:"namespace"`
	Job       string `json:"job"`
	Scope     string `json:"scope"`
	Coverage  struct {
		Total     int  `json:"total"`
		Reachable int  `json:"reachable"`
		Reporting int  `json:"reporting"`
		Complete  bool `json:"complete"`
	} `json:"coverage"`
	Truncated bool           `json:"truncated"`
	Nodes     []string       `json:"nodes"`
	Counts    map[string]int `json:"counts"`
	Findings  []struct {
		Node     string `json:"node"`
		Code     string `json:"code"`
		Evidence string `json:"evidence"`
	} `json:"findings"`
	Events []json.RawMessage `json:"events"`
}

// partial reports whether the view must not be read as complete.
func (r *report) partial() bool { return !r.Coverage.Complete || r.Truncated }

// statusError turns a non-200 gateway answer into an actionable message.
func statusError(code int, detail string) error {
	hint := ""
	switch code {
	case http.StatusUnauthorized:
		hint = " (the token was rejected)"
	case http.StatusForbidden:
		hint = " (this token may not read that namespace)"
	case http.StatusNotFound:
		hint = " (gateway route not found; is the gateway up to date?)"
	case http.StatusServiceUnavailable:
		hint = " (Flight Recorder token unset, or no collector reachable)"
	}
	if detail != "" {
		return fmt.Errorf("gateway returned HTTP %d%s: %s", code, hint, detail)
	}
	return fmt.Errorf("gateway returned HTTP %d%s", code, hint)
}

// fetch returns the raw report body after validating it is a JSON object.
func fetch(ctx context.Context, client *http.Client, gateway, namespace, job, token string) ([]byte, error) {
	u, err := url.Parse(gateway)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("gateway must be an HTTP(S) URL without credentials, query or fragment")
	}
	// The bearer token may only cross plaintext on an explicit localhost tunnel.
	if u.Scheme != "https" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
		return nil, errors.New("use HTTPS or a localhost port-forward for the gateway")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("empty token")
	}
	if len(job) > 63 || len(namespace) > 63 || !dnsLabel.MatchString(job) || !dnsLabel.MatchString(namespace) {
		return nil, errors.New("namespace and job must be DNS-1123 labels")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/flight/jobs/" + job
	u.RawPath = ""
	q := url.Values{}
	q.Set("namespace", namespace)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	// Never forward the credential to a redirect target (for example a login page).
	safe := *client
	safe.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := safe.Do(req)
	if err != nil {
		// url.Error text contains the URL only, never headers.
		return nil, fmt.Errorf("gateway request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, maxErrorBody))
		var e struct {
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(raw, &e) // the gateway's detail is a plain sentence; anything else is dropped
		return nil, statusError(res.StatusCode, strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f {
				return ' '
			}
			return r
		}, e.Detail))
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxReport+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxReport {
		return nil, errors.New("gateway report exceeds 4 MiB")
	}
	var probe map[string]json.RawMessage
	if json.Unmarshal(body, &probe) != nil {
		return nil, errors.New("gateway returned an invalid report")
	}
	return body, nil
}

// printText writes a short human summary. Untrusted strings are printed with %q so control
// characters in event or finding text cannot alter the terminal.
func printText(out io.Writer, r *report) {
	fmt.Fprintf(out, "Job %s/%s\n", r.Namespace, r.Job)
	c := r.Coverage
	fmt.Fprintf(out, "Collectors: %d reachable of %d discovered, %d returned events for this job\n", c.Reachable, c.Total, c.Reporting)
	if r.partial() {
		why := []string{}
		if !c.Complete {
			why = append(why, "not every discovered collector answered")
		}
		if r.Truncated {
			why = append(why, "events were dropped by a limit")
		}
		fmt.Fprintf(out, "PARTIAL VIEW: %s. Missing events do not establish healthy behavior.\n", strings.Join(why, "; "))
	} else {
		fmt.Fprintln(out, "Every discovered collector answered. Nodes without a running collector are not counted.")
	}
	kinds := make([]string, 0, len(r.Counts))
	for k := range r.Counts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	fmt.Fprintf(out, "Events: %d returned\n", len(r.Events))
	for _, k := range kinds {
		fmt.Fprintf(out, "  %-24q %d\n", k, r.Counts[k])
	}
	if len(r.Findings) == 0 {
		fmt.Fprintln(out, "Findings: none in the retained sample")
		return
	}
	fmt.Fprintf(out, "Findings: %d\n", len(r.Findings))
	for _, f := range r.Findings {
		fmt.Fprintf(out, "  %q %q: %q\n", f.Node, f.Code, f.Evidence)
	}
}

func readToken(file string, stdin io.Reader, getenv func(string) string) (string, error) {
	var raw []byte
	var err error
	switch {
	case file == "-":
		raw, err = io.ReadAll(io.LimitReader(stdin, 8193))
	case file != "":
		var f *os.File
		if f, err = os.Open(file); err == nil {
			raw, err = io.ReadAll(io.LimitReader(f, 8193))
			f.Close()
		}
	default:
		raw = []byte(getenv(tokenEnv))
	}
	if err != nil {
		return "", fmt.Errorf("cannot read token: %w", err)
	}
	if len(raw) > 8192 {
		return "", errors.New("token is too long")
	}
	t := strings.TrimSpace(string(raw))
	if t == "" {
		return "", fmt.Errorf("no token: use --token-file (or - for stdin) or set %s", tokenEnv)
	}
	return t, nil
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string, client *http.Client) int {
	fs := flag.NewFlagSet("gryvia-flight", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: gryvia-flight -namespace NS -job JOB [-token-file FILE|-] [flags]\n\n"+
			"Fetches the Flight Recorder report from the Gryvia API gateway. The token is the gateway\n"+
			"API key or a session/OIDC token (not the collector token); it is read from a file, stdin\n"+
			"or $%s, never from a flag value.\n\nFlags:\n", tokenEnv)
		fs.PrintDefaults()
	}
	gateway := fs.String("gateway", "https://localhost:8080", "API gateway URL (HTTPS, or http on localhost via port-forward)")
	namespace := fs.String("namespace", "", "job namespace (required)")
	job := fs.String("job", "", "job name (required)")
	tokenFile := fs.String("token-file", "", "file holding the gateway token, or - for stdin (default: $"+tokenEnv+")")
	output := fs.String("o", "json", "output format: json or text")
	timeout := fs.Duration("timeout", 30*time.Second, "request timeout (the gateway's fan-out may take up to 10s)")
	requireComplete := fs.Bool("require-complete", false, "exit 3 when the report is partial or truncated")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	usage := func(msg string) int { fmt.Fprintln(stderr, "gryvia-flight:", msg); return 2 }
	if fs.NArg() != 0 {
		return usage("unexpected arguments")
	}
	if *namespace == "" || *job == "" {
		return usage("-namespace and -job are required")
	}
	if *output != "json" && *output != "text" {
		return usage("-o must be json or text")
	}
	if *timeout <= 0 {
		return usage("-timeout must be positive")
	}
	token, err := readToken(*tokenFile, stdin, getenv)
	if err != nil {
		return usage(err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	body, err := fetch(ctx, client, *gateway, *namespace, *job, token)
	if err != nil {
		fmt.Fprintln(stderr, "gryvia-flight:", err)
		return 1
	}
	var r report
	if err := json.Unmarshal(body, &r); err != nil {
		fmt.Fprintln(stderr, "gryvia-flight: gateway returned an unexpected report shape")
		return 1
	}
	if *output == "text" {
		printText(stdout, &r)
	} else {
		fmt.Fprintln(stdout, strings.TrimRight(string(body), "\r\n"))
		if r.partial() {
			fmt.Fprintln(stderr, "gryvia-flight: warning: partial or truncated view (see coverage and truncated)")
		}
	}
	if r.partial() && *requireComplete {
		return 3
	}
	return 0
}

func main() {
	// Default transport: TLS verification on, system roots, proxy from the environment.
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv, &http.Client{Timeout: 60 * time.Second}))
}
