package vt

// FoseSearchResponse is the public course-search response shape.
//
// fose responses are plain JSON and are safe to fetch without authentication.
// SeatHawk will eventually use this as the polling source for seat status.
type FoseSearchResponse struct {
	SrcDB   string       `json:"srcdb"`
	Count   int          `json:"count"`
	Results []FoseResult `json:"results"`
}

// FoseResult is one public course-search result.
//
// Stat is VT's compact status code: A=open, F=full, W=waitlist, C=canceled.
// Most fields stay strings because VT returns them as strings.
type FoseResult struct {
	CRN       string `json:"crn"`
	Code      string `json:"code"`
	Title     string `json:"title"`
	Stat      string `json:"stat"`
	Total     string `json:"total"`
	HoursHTML string `json:"hours_html"`
	SrcDB     string `json:"srcdb"`
}

// StudentData is the minimal studentdata shape SeatHawk needs initially.
//
// studentdata is the source of truth for authenticated state. Registered is
// keyed by term code and contains pipe-delimited section strings; Cart contains
// pipe-delimited cart strings; RegTickets contains the server-provided ticket
// pieces needed later for shockabsorber time_ticket construction.
type StudentData struct {
	Pers       Person               `json:"pers"`
	Cart       []string             `json:"cart"`
	Registered map[string][]string  `json:"reg"`
	RegTickets []RegistrationTicket `json:"reg_tickets"`
}

// CartResponse is the cart_read response shape returned by sisproxy.
//
// The cart array intentionally stores raw strings because VT encodes each row
// as an 18-field pipe-delimited record. Use ParseCartEntry for field access.
type CartResponse struct {
	Cart []string `json:"cart"`
}

// CartAddInput is the sisproxy cart_add request for staging one add CRN.
//
// Cart staging is a VT write operation. The registration package uses it only
// after fresh safety checks prove the add is currently allowed to proceed.
type CartAddInput struct {
	Authtoken string
	Term      string
	CRN       string
	Hours     string
	GradeMode string
	RegInfo   string
}

// Person is the minimal authenticated-person shape from studentdata.
//
// ID and IDProof are not Banner IDs. They are opaque signed values that later
// shockabsorber calls require alongside authtoken.
type Person struct {
	FirstName string `json:"fn"`
	ID        string `json:"id"`
	IDProof   string `json:"idProof"`
	Class     string `json:"clas"`
}

// PreflightResponse is the validation response returned by sisproxy preflight.
//
// RegCourseErrors is keyed by CRN. VT commonly returns strings like "||" for
// no actionable course error, while real blockers include text separated by
// pipes/newlines. RegNonCourseErrors holds global blockers.
type PreflightResponse struct {
	RegCourseErrors    map[string]string `json:"reg_course_errors"`
	RegNonCourseErrors []string          `json:"reg_non-course_errors"`
}

// ShockabsorberCredentials are the identity fields VT requires in POST bodies.
//
// authtoken authenticates the browser session; PersonID and PersonIDProof come
// from fresh studentdata and prove which student record shockabsorber should
// submit against.
type ShockabsorberCredentials struct {
	Authtoken     string
	PersonID      string
	PersonIDProof string
}

// ShockabsorberRegisterInput is the actual Banner registration queue request.
//
// TimeTicket and URLReplay are placed in the query string by the client.
// Credentials are sent in the form body, matching the browser's request shape.
type ShockabsorberRegisterInput struct {
	Credentials ShockabsorberCredentials
	TimeTicket  string
	URLReplay   string
}

// ShockabsorberStatusInput checks the queue status for a submitted registration.
//
// It intentionally does not include URLReplay because status polling follows an
// already-submitted time_ticket rather than replaying the register URL again.
type ShockabsorberStatusInput struct {
	Credentials ShockabsorberCredentials
	TimeTicket  string
}

// ShockabsorberResponse is the flexible response envelope from shockabsorber.
//
// Body carries statuses such as WAIT, OK, or PROCESSED. Data is intentionally a
// generic object because VT's nested registration result payloads vary by
// outcome and still need more empirical examples.
type ShockabsorberResponse struct {
	Body string         `json:"body"`
	Code int            `json:"code"`
	Data map[string]any `json:"data"`
}
