package cli

import (
	"fmt"
	"strconv"

	"github.com/brennanhumphrey/seathawk/internal/vt"
	watchsvc "github.com/brennanhumphrey/seathawk/internal/watch"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Manage local registration watches",
		Long:  "Manage local watch definitions. Creation and polling may contact VT for read-only checks, but watch commands do not attempt registration.",
	}

	cmd.AddCommand(newWatchAddCmd())
	cmd.AddCommand(newWatchSwapCmd())
	cmd.AddCommand(newWatchListCmd())
	cmd.AddCommand(newWatchPollCmd())
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
			return withWatchService(cmd, func(svc watchsvc.Service) error {
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
			})
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
			return withWatchService(cmd, func(svc watchsvc.Service) error {
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
			})
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
			return withWatchService(cmd, func(svc watchsvc.Service) error {
				watches, err := svc.List(cmd.Context())
				if err != nil {
					return err
				}
				printWatchList(cmd.OutOrStdout(), watches)
				return nil
			})
		},
	}
	return cmd
}

func newWatchPollCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "poll",
		Short: "Run one read-only polling pass",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withWatchService(cmd, func(svc watchsvc.Service) error {
				report, err := svc.PollDue(cmd.Context(), watchsvc.PollInput{All: all})
				if err != nil {
					return err
				}
				printPollReport(cmd.OutOrStdout(), report)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Poll all active watches, ignoring next_poll_at")
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
			return withWatchService(cmd, func(svc watchsvc.Service) error {
				watch, err := svc.Enable(cmd.Context(), id)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "watch enabled")
				printWatchSummary(cmd.OutOrStdout(), watch)
				return nil
			})
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
			return withWatchService(cmd, func(svc watchsvc.Service) error {
				watch, err := svc.Disable(cmd.Context(), id)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "watch disabled")
				printWatchSummary(cmd.OutOrStdout(), watch)
				return nil
			})
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
			return withWatchService(cmd, func(svc watchsvc.Service) error {
				if err := svc.Remove(cmd.Context(), id); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "watch removed: %d\n", id)
				return nil
			})
		},
	}
	return cmd
}

func withWatchService(cmd *cobra.Command, fn func(watchsvc.Service) error) error {
	svc, cleanup, err := newWatchService(cmd)
	if err != nil {
		return err
	}
	defer cleanup()
	return fn(svc)
}

func newWatchService(cmd *cobra.Command) (watchsvc.Service, func(), error) {
	_, db, cleanup, err := openAppDB(cmd)
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
