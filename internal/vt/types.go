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
