package vt

import (
	"fmt"
	"net/url"
	"strings"
)

// BuildSingleReplayURL builds the relative sisproxy replay URL for one CRN.
//
// This creates the inner url_replay value used later by shockabsorber. It is
// relative on purpose: the browser sends "api/?..." rather than a full URL.
// Do not include authtoken here; VT's Banner path sends auth in the outer
// shockabsorber form body, not inside url_replay.
func BuildSingleReplayURL(term, crn string) (string, error) {
	term, crn, err := requireTermAndCRN(term, crn)
	if err != nil {
		return "", err
	}

	return replayURL(term, url.QueryEscape(crn)), nil
}

// BuildSwapReplayURL builds the relative sisproxy replay URL for an add/drop swap.
//
// For swaps, VT requires both CRNs in the replay URL's crn parameter. The
// cart_add crn_drop parameter only stages intent; it is not enough to make the
// later registration transaction a swap.
func BuildSwapReplayURL(term, addCRN, dropCRN string) (string, error) {
	term, addCRN, err := requireTermAndCRN(term, addCRN)
	if err != nil {
		return "", err
	}
	dropCRN = strings.TrimSpace(dropCRN)
	if dropCRN == "" {
		return "", fmt.Errorf("drop CRN is empty")
	}

	// The comma is encoded in the inner replay URL so outer query encoding later
	// turns it into %252C, matching the browser's shockabsorber request shape.
	crns := url.QueryEscape(addCRN + "," + dropCRN)
	return replayURL(term, crns), nil
}

func requireTermAndCRN(term, crn string) (string, string, error) {
	term = strings.TrimSpace(term)
	crn = strings.TrimSpace(crn)
	if term == "" {
		return "", "", fmt.Errorf("term is empty")
	}
	if crn == "" {
		return "", "", fmt.Errorf("CRN is empty")
	}
	return term, crn, nil
}

func replayURL(term, encodedCRN string) string {
	// Empty wait_crn and swap_crn parameters are intentionally present because
	// browser-issued requests include them and shockabsorber replays this URL.
	return "api/?page=sisproxy&action=register&term_code=" +
		url.QueryEscape(term) +
		"&crn=" + encodedCRN +
		"&wait_crn=&swap_crn="
}
