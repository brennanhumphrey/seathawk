package vt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL       = "https://classes.vt.edu"
	defaultClientTimeout = 15 * time.Second
)

// ClientConfig configures a VT HTTP client at construction time.
//
// BaseURL is a string because tests and future app config naturally provide
// URLs as text. NewClient parses and validates it once, then stores the
// normalized *url.URL on Client.
type ClientConfig struct {
	BaseURL    string
	HTTPClient *http.Client
}

// Client performs safe read-only requests against the VT registration API.
//
// The fields are intentionally unexported. Once NewClient has validated the
// base URL and chosen an HTTP client, callers should use methods instead of
// mutating runtime state underneath in-flight requests.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

// HTTPError describes a non-successful VT HTTP response.
//
// Authenticated sisproxy requests carry authtoken in the query string, so this
// error deliberately records only the logical operation and status code. Do not
// add full request URLs or response bodies here unless they are explicitly
// redacted first.
type HTTPError struct {
	Operation  string
	StatusCode int
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("%s returned HTTP %d", e.Operation, e.StatusCode)
}

// NewClient constructs a VT client with validated defaults.
//
// An empty BaseURL targets the real VT host. Tests should pass an httptest
// server URL so they can verify request shape without touching classes.vt.edu.
func NewClient(cfg ClientConfig) (*Client, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("base URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("base URL must include a host")
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultClientTimeout}
	}

	return &Client{
		baseURL:    parsed,
		httpClient: httpClient,
	}, nil
}

// SearchByCRN searches the public fose endpoint for one CRN in one term.
//
// fose is the only VT endpoint used here that returns plain JSON and does not
// require an authtoken. It is the safe polling source for seat status.
func (c *Client) SearchByCRN(ctx context.Context, term, crn string) (FoseSearchResponse, error) {
	term, err := requireValue("term", term)
	if err != nil {
		return FoseSearchResponse{}, err
	}
	crn, err = requireValue("CRN", crn)
	if err != nil {
		return FoseSearchResponse{}, err
	}

	body := foseSearchRequest{
		Other: map[string]string{"srcdb": term},
		Criteria: []foseCriterion{
			{Field: "crn", Value: crn},
		},
	}

	var out FoseSearchResponse
	if err := c.doJSON(ctx, http.MethodPost, "fose search", url.Values{
		"page":  {"fose"},
		"route": {"search"},
	}, body, &out); err != nil {
		return FoseSearchResponse{}, err
	}

	return out, nil
}

// StudentData fetches the authenticated student record from sisproxy.
//
// This is the authoritative authenticated snapshot: identity, current cart,
// registered sections, and registration tickets all come from this one call.
// The response is JSONP wrapped as setRecord(...), not plain JSON.
func (c *Client) StudentData(ctx context.Context, authtoken string) (StudentData, error) {
	authtoken, err := requireValue("authtoken", authtoken)
	if err != nil {
		return StudentData{}, err
	}

	var out StudentData
	if err := c.doJSONP(ctx, "studentdata", url.Values{
		"page":      {"sisproxy"},
		"action":    {"studentdata"},
		"authtoken": {authtoken},
	}, &out); err != nil {
		return StudentData{}, err
	}

	return out, nil
}

// CartRead fetches the authenticated default cart from sisproxy.
//
// This method only reads cart state. Cart entries remain raw pipe-delimited
// strings here; callers can pass them to ParseCartEntry when they need fields.
// The response is JSONP wrapped as setCart(...).
func (c *Client) CartRead(ctx context.Context, authtoken string) (CartResponse, error) {
	authtoken, err := requireValue("authtoken", authtoken)
	if err != nil {
		return CartResponse{}, err
	}

	var out CartResponse
	if err := c.doJSONP(ctx, "cart_read", url.Values{
		"page":      {"sisproxy"},
		"action":    {"cart_read"},
		"authtoken": {authtoken},
	}, &out); err != nil {
		return CartResponse{}, err
	}

	return out, nil
}

// Preflight validates CRNs against the authenticated student's registration state.
//
// Preflight can surface prerequisite, conflict, restriction, and registration
// window errors before any cart or registration write. It still does not prove a
// later registration will succeed, so later phases must confirm final state with
// StudentData after any actual registration attempt.
func (c *Client) Preflight(ctx context.Context, authtoken, term string, crns []string) (PreflightResponse, error) {
	authtoken, err := requireValue("authtoken", authtoken)
	if err != nil {
		return PreflightResponse{}, err
	}
	term, err = requireValue("term", term)
	if err != nil {
		return PreflightResponse{}, err
	}
	crnList, err := joinCRNs(crns)
	if err != nil {
		return PreflightResponse{}, err
	}

	var out PreflightResponse
	if err := c.doJSONP(ctx, "preflight", url.Values{
		"page":      {"sisproxy"},
		"action":    {"preflight"},
		"term_code": {term},
		"cart_name": {"default"},
		"crn_list":  {crnList},
		"authtoken": {authtoken},
	}, &out); err != nil {
		return PreflightResponse{}, err
	}

	return out, nil
}

type foseSearchRequest struct {
	Other    map[string]string `json:"other"`
	Criteria []foseCriterion   `json:"criteria"`
}

type foseCriterion struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// doJSON handles the public/plain-JSON side of VT's API. Today that is fose.
func (c *Client) doJSON(ctx context.Context, method, operation string, query url.Values, requestBody any, target any) error {
	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("marshal %s request: %w", operation, err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.apiURL(query), body)
	if err != nil {
		return fmt.Errorf("build %s request: %w", operation, err)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s request: %w", operation, err)
	}
	defer resp.Body.Close()

	if err := checkStatus(operation, resp); err != nil {
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s response: %w", operation, err)
	}

	return nil
}

// doJSONP handles sisproxy responses. VT wraps these as function calls like
// setRecord({...}) or preflight({...}), so they must be unwrapped before JSON
// unmarshalling.
func (c *Client) doJSONP(ctx context.Context, operation string, query url.Values, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL(query), nil)
	if err != nil {
		return fmt.Errorf("build %s request: %w", operation, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s request: %w", operation, err)
	}
	defer resp.Body.Close()

	if err := checkStatus(operation, resp); err != nil {
		return err
	}

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read %s response: %w", operation, err)
	}
	jsonPayload, err := StripJSONP(string(contents))
	if err != nil {
		return fmt.Errorf("decode %s JSONP: %w", operation, err)
	}
	if err := json.Unmarshal(jsonPayload, target); err != nil {
		return fmt.Errorf("decode %s response: %w", operation, err)
	}

	return nil
}

func (c *Client) apiURL(query url.Values) string {
	u := *c.baseURL
	// VT multiplexes fose, sisproxy, and shockabsorber behind /api/ and chooses
	// behavior from query parameters like page=sisproxy&action=studentdata.
	u.Path = strings.TrimRight(u.Path, "/") + "/api/"
	u.RawQuery = query.Encode()
	return u.String()
}

func checkStatus(operation string, resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Drain the body for connection reuse, but do not include it in the error:
	// VT error pages can reflect credential-bearing query strings.
	_, _ = io.Copy(io.Discard, resp.Body)
	return HTTPError{Operation: operation, StatusCode: resp.StatusCode}
}

func requireValue(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is empty", name)
	}
	return value, nil
}

func joinCRNs(crns []string) (string, error) {
	if len(crns) == 0 {
		return "", fmt.Errorf("CRN list is empty")
	}

	trimmed := make([]string, 0, len(crns))
	for _, crn := range crns {
		value := strings.TrimSpace(crn)
		if value == "" {
			return "", fmt.Errorf("CRN list contains an empty value")
		}
		trimmed = append(trimmed, value)
	}

	// sisproxy preflight expects one comma-separated crn_list query value.
	return strings.Join(trimmed, ","), nil
}
