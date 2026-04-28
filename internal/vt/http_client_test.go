package vt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClientDefaults(t *testing.T) {
	client, err := NewClient(ClientConfig{})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if client.baseURL.String() != defaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", client.baseURL.String(), defaultBaseURL)
	}
	if client.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
}

func TestNewClientWithTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if client.baseURL.String() != server.URL {
		t.Fatalf("baseURL = %q, want %q", client.baseURL.String(), server.URL)
	}
}

func TestNewClientRejectsInvalidBaseURL(t *testing.T) {
	tests := []string{
		"://bad",
		"classes.vt.edu",
		"ftp://classes.vt.edu",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := NewClient(ClientConfig{BaseURL: input}); err == nil {
				t.Fatal("NewClient returned nil error")
			}
		})
	}
}

func TestSearchByCRN(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/" {
			t.Fatalf("path = %s, want /api/", r.URL.Path)
		}
		if got := r.URL.Query().Get("page"); got != "fose" {
			t.Fatalf("page = %q, want fose", got)
		}
		if got := r.URL.Query().Get("route"); got != "search" {
			t.Fatalf("route = %q, want search", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}

		var body struct {
			Other    map[string]string `json:"other"`
			Criteria []struct {
				Field string `json:"field"`
				Value string `json:"value"`
			} `json:"criteria"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Other["srcdb"] != "202606" {
			t.Fatalf("srcdb = %q, want 202606", body.Other["srcdb"])
		}
		if len(body.Criteria) != 1 || body.Criteria[0].Field != "crn" || body.Criteria[0].Value != "60058" {
			t.Fatalf("criteria = %+v, want crn=60058", body.Criteria)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"srcdb":"202606","count":1,"results":[{"crn":"60058","code":"AAEC 2104","title":"Hunger Issues","stat":"A","total":"30","hours_html":"3 Credit Hours","srcdb":"202606"}]}`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	got, err := client.SearchByCRN(context.Background(), "202606", "60058")
	if err != nil {
		t.Fatalf("SearchByCRN returned error: %v", err)
	}
	if got.Count != 1 || got.Results[0].CRN != "60058" || got.Results[0].Stat != "A" {
		t.Fatalf("unexpected search response: %+v", got)
	}
}

func TestStudentData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireGETQuery(t, r, map[string]string{
			"page":      "sisproxy",
			"action":    "studentdata",
			"authtoken": "secret-token",
		})
		_, _ = w.Write([]byte(`setRecord({"pers":{"fn":"Redacted","id":"person-id","idProof":"person-proof","clas":"10"},"cart":[],"reg":{"202606":["60900|CS 3304||N|3|UG|misc"]},"reg_tickets":[]});`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	got, err := client.StudentData(context.Background(), "secret-token")
	if err != nil {
		t.Fatalf("StudentData returned error: %v", err)
	}
	if got.Pers.ID != "person-id" || got.Registered["202606"][0] == "" {
		t.Fatalf("unexpected student data: %+v", got)
	}
}

func TestCartRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireGETQuery(t, r, map[string]string{
			"page":      "sisproxy",
			"action":    "cart_read",
			"authtoken": "secret-token",
		})
		_, _ = w.Write([]byte(`setCart({"cart":["202606|default|60058|3||||AAEC 2104||N|||E|||||"]})`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	got, err := client.CartRead(context.Background(), "secret-token")
	if err != nil {
		t.Fatalf("CartRead returned error: %v", err)
	}
	if len(got.Cart) != 1 {
		t.Fatalf("cart length = %d, want 1", len(got.Cart))
	}
}

func TestCartAdd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireGETQuery(t, r, map[string]string{
			"page":      "sisproxy",
			"action":    "cart_add",
			"term_code": "202606",
			"cart_name": "default",
			"crn":       "60058",
			"hours":     "3",
			"gmod":      "N",
			"reg_info":  "E",
			"authtoken": "secret-token",
		})
		_, _ = w.Write([]byte(`setCart({"cart":["202606|default|60058|3||||AAEC 2104||N|||E|||||"]})`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	got, err := client.CartAdd(context.Background(), CartAddInput{
		Authtoken: "secret-token",
		Term:      "202606",
		CRN:       "60058",
		Hours:     "3",
		GradeMode: "N",
		RegInfo:   "E",
	})
	if err != nil {
		t.Fatalf("CartAdd returned error: %v", err)
	}
	if len(got.Cart) != 1 {
		t.Fatalf("cart length = %d, want 1", len(got.Cart))
	}
}

func TestPreflight(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireGETQuery(t, r, map[string]string{
			"page":      "sisproxy",
			"action":    "preflight",
			"term_code": "202606",
			"cart_name": "default",
			"crn_list":  "60058,60900",
			"authtoken": "secret-token",
		})
		_, _ = w.Write([]byte(`preflight({"reg_course_errors":{"60058":"||"},"reg_non-course_errors":[]})`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	got, err := client.Preflight(context.Background(), "secret-token", "202606", []string{"60058", "60900"})
	if err != nil {
		t.Fatalf("Preflight returned error: %v", err)
	}
	if got.RegCourseErrors["60058"] != "||" || len(got.RegNonCourseErrors) != 0 {
		t.Fatalf("unexpected preflight response: %+v", got)
	}
}

func TestShockabsorberRegister(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requirePOSTForm(t, r, map[string]string{
			"authtoken":      "secret-token",
			"_pers_id":       "person-id",
			"_pers_id_proof": "person-proof",
			"_pers_real_id":  "person-id",
		})
		query := r.URL.Query()
		for key, want := range map[string]string{
			"page":        "shockabsorber",
			"time_ticket": "ticket|person-id",
			"action":      "register",
			"cart_name":   "default",
			"url_replay":  "api/?page=sisproxy&action=register&term_code=202606&crn=60058&wait_crn=&swap_crn=",
		} {
			if got := query.Get(key); got != want {
				t.Fatalf("%s = %q, want %q", key, got, want)
			}
		}

		_, _ = w.Write([]byte(`{"body":"WAIT","code":200,"data":{"id":"144260"}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	got, err := client.ShockabsorberRegister(context.Background(), ShockabsorberRegisterInput{
		Credentials: ShockabsorberCredentials{
			Authtoken:     "secret-token",
			PersonID:      "person-id",
			PersonIDProof: "person-proof",
		},
		TimeTicket: "ticket|person-id",
		URLReplay:  "api/?page=sisproxy&action=register&term_code=202606&crn=60058&wait_crn=&swap_crn=",
	})
	if err != nil {
		t.Fatalf("ShockabsorberRegister returned error: %v", err)
	}
	if got.Body != "WAIT" || got.Code != 200 {
		t.Fatalf("unexpected shockabsorber response: %+v", got)
	}
}

func TestShockabsorberStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requirePOSTForm(t, r, map[string]string{
			"authtoken":      "secret-token",
			"_pers_id":       "person-id",
			"_pers_id_proof": "person-proof",
			"_pers_real_id":  "person-id",
		})
		query := r.URL.Query()
		for key, want := range map[string]string{
			"page":        "shockabsorber",
			"time_ticket": "ticket|person-id",
			"action":      "status",
			"cart_name":   "default",
		} {
			if got := query.Get(key); got != want {
				t.Fatalf("%s = %q, want %q", key, got, want)
			}
		}
		if got := query.Get("url_replay"); got != "" {
			t.Fatalf("url_replay = %q, want empty", got)
		}

		_, _ = w.Write([]byte(`{"body":"PROCESSED","code":200,"data":{"reg_success":["60058"]}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server)
	got, err := client.ShockabsorberStatus(context.Background(), ShockabsorberStatusInput{
		Credentials: ShockabsorberCredentials{
			Authtoken:     "secret-token",
			PersonID:      "person-id",
			PersonIDProof: "person-proof",
		},
		TimeTicket: "ticket|person-id",
	})
	if err != nil {
		t.Fatalf("ShockabsorberStatus returned error: %v", err)
	}
	if got.Body != "PROCESSED" || got.Code != 200 {
		t.Fatalf("unexpected shockabsorber response: %+v", got)
	}
}

func TestClientValidationDoesNotMakeRequest(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	client := newTestClient(t, server)
	cases := []struct {
		name string
		call func() error
	}{
		{name: "search empty term", call: func() error { _, err := client.SearchByCRN(context.Background(), "", "60058"); return err }},
		{name: "search empty crn", call: func() error { _, err := client.SearchByCRN(context.Background(), "202606", ""); return err }},
		{name: "studentdata empty token", call: func() error { _, err := client.StudentData(context.Background(), ""); return err }},
		{name: "cart empty token", call: func() error { _, err := client.CartRead(context.Background(), ""); return err }},
		{name: "cart add missing hours", call: func() error {
			_, err := client.CartAdd(context.Background(), CartAddInput{Authtoken: "token", Term: "202606", CRN: "60058", GradeMode: "N", RegInfo: "E"})
			return err
		}},
		{name: "preflight empty term", call: func() error {
			_, err := client.Preflight(context.Background(), "token", "", []string{"60058"})
			return err
		}},
		{name: "preflight empty crns", call: func() error { _, err := client.Preflight(context.Background(), "token", "202606", nil); return err }},
		{name: "preflight empty crn value", call: func() error {
			_, err := client.Preflight(context.Background(), "token", "202606", []string{"60058", ""})
			return err
		}},
		{name: "shockabsorber register missing replay", call: func() error {
			_, err := client.ShockabsorberRegister(context.Background(), ShockabsorberRegisterInput{
				Credentials: ShockabsorberCredentials{Authtoken: "token", PersonID: "person", PersonIDProof: "proof"},
				TimeTicket:  "ticket",
			})
			return err
		}},
		{name: "shockabsorber status missing proof", call: func() error {
			_, err := client.ShockabsorberStatus(context.Background(), ShockabsorberStatusInput{
				Credentials: ShockabsorberCredentials{Authtoken: "token", PersonID: "person"},
				TimeTicket:  "ticket",
			})
			return err
		}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			if err := tt.call(); err == nil {
				t.Fatal("call returned nil error")
			}
			if called {
				t.Fatal("request was made despite validation error")
			}
		})
	}
}

func TestClientHTTPErrorDoesNotLeakAuthtoken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad token secret-token", http.StatusForbidden)
	}))
	defer server.Close()

	client := newTestClient(t, server)
	_, err := client.StudentData(context.Background(), "secret-token")
	if err == nil {
		t.Fatal("StudentData returned nil error")
	}

	var httpErr HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want HTTPError", err)
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error leaked token: %v", err)
	}
	if httpErr.StatusCode != http.StatusForbidden || httpErr.Operation != "studentdata" {
		t.Fatalf("unexpected HTTPError: %+v", httpErr)
	}
}

func TestClientMalformedResponses(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
		body string
	}{
		{name: "plain json malformed", body: `{`, call: func(c *Client) error {
			_, err := c.SearchByCRN(context.Background(), "202606", "60058")
			return err
		}},
		{name: "jsonp malformed wrapper", body: `{"cart":[]}`, call: func(c *Client) error {
			_, err := c.CartRead(context.Background(), "token")
			return err
		}},
		{name: "jsonp malformed payload", body: `setCart({)`, call: func(c *Client) error {
			_, err := c.CartRead(context.Background(), "token")
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := newTestClient(t, server)
			if err := tt.call(client); err == nil {
				t.Fatal("call returned nil error")
			}
		})
	}
}

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	client, err := NewClient(ClientConfig{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	return client
}

func requireGETQuery(t *testing.T, r *http.Request, want map[string]string) {
	t.Helper()

	if r.Method != http.MethodGet {
		t.Fatalf("method = %s, want GET", r.Method)
	}
	if r.URL.Path != "/api/" {
		t.Fatalf("path = %s, want /api/", r.URL.Path)
	}

	query := r.URL.Query()
	for key, value := range want {
		if got := query.Get(key); got != value {
			t.Fatalf("%s = %q, want %q", key, got, value)
		}
	}
}

func requirePOSTForm(t *testing.T, r *http.Request, want map[string]string) {
	t.Helper()

	if r.Method != http.MethodPost {
		t.Fatalf("method = %s, want POST", r.Method)
	}
	if r.URL.Path != "/api/" {
		t.Fatalf("path = %s, want /api/", r.URL.Path)
	}
	if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q, want application/x-www-form-urlencoded", got)
	}
	if err := r.ParseForm(); err != nil {
		t.Fatalf("ParseForm returned error: %v", err)
	}
	for key, value := range want {
		if got := r.Form.Get(key); got != value {
			t.Fatalf("form %s = %q, want %q", key, got, value)
		}
	}
}
