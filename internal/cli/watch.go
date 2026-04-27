package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/vt"
	watchsvc "github.com/brennanhumphrey/seathawk/internal/watch"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Manage local registration watches",
		Long:  "Manage local watch definitions. These commands do not contact VT or attempt registration.",
	}

	cmd.AddCommand(newWatchAddCmd())
	cmd.AddCommand(newWatchSwapCmd())
	cmd.AddCommand(newWatchListCmd())
	cmd.AddCommand(newWatchEnableCmd())
	cmd.AddCommand(newWatchDisableCmd())
	cmd.AddCommand(newWatchRemoveCmd())
	return cmd
}

func newWatchAddCmd() *cobra.Command {
	var term string
	var crn string

	cmd := &cobra.Command{
		Use:   "add --term <term> --crn <crn>",
		Short: "Create a local add watch",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := newWatchService(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			// CLI commands pass raw flag strings through to the service so all
			// normalization and validation lives in one place.
			watch, evaluation, err := svc.Add(cmd.Context(), watchsvc.CreateAddInput{Term: term, CRN: crn})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "watch created")
			printWatchSummary(cmd.OutOrStdout(), watch)
			printEvaluationSummary(cmd.OutOrStdout(), evaluation)
			return nil
		},
	}
	cmd.Flags().StringVar(&term, "term", "", "Six-digit VT term code")
	cmd.Flags().StringVar(&crn, "crn", "", "Five-digit CRN to add")
	_ = cmd.MarkFlagRequired("term")
	_ = cmd.MarkFlagRequired("crn")
	return cmd
}

func newWatchSwapCmd() *cobra.Command {
	var term string
	var addCRN string
	var dropCRN string

	cmd := &cobra.Command{
		Use:   "swap --term <term> --add-crn <crn> --drop-crn <crn>",
		Short: "Create a local swap watch",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := newWatchService(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			// Swap is intentionally a separate command because later phases will
			// make it a potentially destructive add/drop workflow.
			watch, evaluation, err := svc.Swap(cmd.Context(), watchsvc.CreateSwapInput{
				Term:    term,
				AddCRN:  addCRN,
				DropCRN: dropCRN,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "watch created")
			printWatchSummary(cmd.OutOrStdout(), watch)
			printEvaluationSummary(cmd.OutOrStdout(), evaluation)
			return nil
		},
	}
	cmd.Flags().StringVar(&term, "term", "", "Six-digit VT term code")
	cmd.Flags().StringVar(&addCRN, "add-crn", "", "Five-digit CRN to add")
	cmd.Flags().StringVar(&dropCRN, "drop-crn", "", "Five-digit CRN to drop")
	_ = cmd.MarkFlagRequired("term")
	_ = cmd.MarkFlagRequired("add-crn")
	_ = cmd.MarkFlagRequired("drop-crn")
	return cmd
}

func newWatchListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local watches",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := newWatchService(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			watches, err := svc.List(cmd.Context())
			if err != nil {
				return err
			}
			printWatchList(cmd.OutOrStdout(), watches)
			return nil
		},
	}
	return cmd
}

func newWatchEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable a local watch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseWatchID(args[0])
			if err != nil {
				return err
			}
			svc, cleanup, err := newWatchService(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			watch, err := svc.Enable(cmd.Context(), id)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "watch enabled")
			printWatchSummary(cmd.OutOrStdout(), watch)
			return nil
		},
	}
	return cmd
}

func newWatchDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable a local watch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseWatchID(args[0])
			if err != nil {
				return err
			}
			svc, cleanup, err := newWatchService(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			watch, err := svc.Disable(cmd.Context(), id)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "watch disabled")
			printWatchSummary(cmd.OutOrStdout(), watch)
			return nil
		},
	}
	return cmd
}

func newWatchRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a local watch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseWatchID(args[0])
			if err != nil {
				return err
			}
			svc, cleanup, err := newWatchService(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			if err := svc.Remove(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "watch removed: %d\n", id)
			return nil
		},
	}
	return cmd
}

func newWatchService(ctx context.Context) (watchsvc.Service, func(), error) {
	_, db, cleanup, err := openAppDB(ctx)
	if err != nil {
		return watchsvc.Service{}, nil, err
	}

	client, err := vt.NewClient(vt.ClientConfig{})
	if err != nil {
		cleanup()
		return watchsvc.Service{}, nil, fmt.Errorf("create VT client: %w", err)
	}

	return watchsvc.Service{DB: db, VTClient: client}, cleanup, nil
}

func parseWatchID(raw string) (int64, error) {
	// IDs come from SQLite autoincrement primary keys, so zero/negative values
	// can never be valid user input.
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("watch id must be a positive integer")
	}
	return id, nil
}

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
