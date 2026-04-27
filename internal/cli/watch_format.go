package cli

import (
	"database/sql"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	watchsvc "github.com/brennanhumphrey/seathawk/internal/watch"
)

func printWatchList(w io.Writer, watches []store.Watch) {
	if len(watches) == 0 {
		fmt.Fprintln(w, "no watches found")
		return
	}

	// Disabled watches stay visible so users can tell whether a desired watch
	// is paused rather than missing.
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tACTIVE\tMODE\tTERM\tADD_CRN\tDROP_CRN\tLAST_SEEN\tNEXT_POLL_AT\tLAST_ATTEMPT_AT")
	for _, watch := range watches {
		fmt.Fprintf(tw, "%d\t%t\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			watch.ID,
			watch.Active,
			watch.Mode,
			watch.Term,
			watch.AddCRN,
			formatNullableString(watch.DropCRN),
			formatNullableString(watch.LastSeenStat),
			formatNullableTime(watch.NextPollAt),
			formatNullableTime(watch.LastAttemptAt),
		)
	}
	_ = tw.Flush()
}

func printWatchSummary(w io.Writer, watch store.Watch) {
	fmt.Fprintf(w, "id: %d\n", watch.ID)
	fmt.Fprintf(w, "active: %t\n", watch.Active)
	fmt.Fprintf(w, "mode: %s\n", watch.Mode)
	fmt.Fprintf(w, "term: %s\n", watch.Term)
	fmt.Fprintf(w, "add_crn: %s\n", watch.AddCRN)
	fmt.Fprintf(w, "drop_crn: %s\n", formatNullableString(watch.DropCRN))
}

func printEvaluationSummary(w io.Writer, evaluation watchsvc.Evaluation) {
	fmt.Fprintf(w, "section_status: %s\n", evaluation.SectionStatus)
	if evaluation.SectionTitle != "" {
		fmt.Fprintf(w, "section_title: %s\n", evaluation.SectionTitle)
	}
	fmt.Fprintf(w, "registration_window: %s\n", evaluation.Window)
	if len(evaluation.Notes) > 0 {
		fmt.Fprintln(w, "notes:")
		for _, note := range evaluation.Notes {
			fmt.Fprintf(w, "- %s\n", note)
		}
	}
	if len(evaluation.HardRejects) > 0 {
		fmt.Fprintln(w, "hard_rejects:")
		for _, reject := range evaluation.HardRejects {
			fmt.Fprintf(w, "- %s\n", reject)
		}
	}
}

func printPollReport(w io.Writer, report watchsvc.PollReport) {
	if len(report.Results) == 0 {
		fmt.Fprintln(w, "no watches due")
		return
	}

	fmt.Fprintf(w, "checked_at: %s\n", report.CheckedAt.UTC().Format(time.RFC3339))
	for i, result := range report.Results {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "watch %d\n", result.Watch.ID)
		printWatchSummary(w, result.Watch)
		if result.Err != nil {
			fmt.Fprintf(w, "error: %v\n", result.Err)
			continue
		}
		printEvaluationSummary(w, result.Evaluation)
	}
}

func formatNullableString(value sql.NullString) string {
	// The CLI uses "-" for unset scheduler fields so tables stay compact and
	// visibly distinguish "not known yet" from an empty string.
	if !value.Valid || value.String == "" {
		return "-"
	}
	return value.String
}

func formatNullableTime(value sql.NullTime) string {
	if !value.Valid {
		return "-"
	}
	return value.Time.UTC().Format(time.RFC3339)
}
