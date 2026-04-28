package cli

import (
	"fmt"
	"io"

	"github.com/brennanhumphrey/seathawk/internal/register"
	"github.com/brennanhumphrey/seathawk/internal/vt"
	"github.com/spf13/cobra"
)

// newRegisterCmd builds commands that can perform explicit VT registration writes.
func newRegisterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Run explicit VT registration attempts",
		Long: "Run explicit VT registration attempts.\n\n" +
			"These commands can submit real registration changes to VT. The daemon\n" +
			"remains read-only; registration attempts only happen when you run a\n" +
			"register command with the required confirmation flag.",
	}

	cmd.AddCommand(newRegisterAttemptCmd())
	return cmd
}

// newRegisterAttemptCmd builds the manual single-add attempt command.
//
// The required --confirm flag makes the dangerous action explicit at the CLI
// boundary before register.Service is allowed to contact VT write endpoints.
func newRegisterAttemptCmd() *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:   "attempt <watch-id>",
		Short: "Attempt one manual add registration for a watch",
		Long: "Attempt one manual add registration for a watch.\n\n" +
			"This command performs fresh safety checks, stages the CRN in VT's\n" +
			"default cart, submits one shockabsorber register call, polls status,\n" +
			"and confirms the result with fresh studentdata. Swap watches are not\n" +
			"supported yet.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				return fmt.Errorf("refusing to attempt registration without --confirm")
			}
			id, err := parseWatchID(args[0])
			if err != nil {
				return err
			}
			return withRegisterService(cmd, func(svc register.Service) error {
				result, err := svc.AttemptAdd(cmd.Context(), register.AttemptAddInput{WatchID: id})
				if err != nil {
					return err
				}
				printAttemptResult(cmd.OutOrStdout(), result)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm that this command may submit a real VT registration attempt")
	return cmd
}

// withRegisterService opens app dependencies for one registration command.
func withRegisterService(cmd *cobra.Command, fn func(register.Service) error) error {
	svc, cleanup, err := newRegisterService(cmd)
	if err != nil {
		return err
	}
	defer cleanup()
	return fn(svc)
}

// newRegisterService wires SQLite and the real VT client into register.Service.
func newRegisterService(cmd *cobra.Command) (register.Service, func(), error) {
	_, db, cleanup, err := openAppDB(cmd)
	if err != nil {
		return register.Service{}, nil, err
	}

	client, err := vt.NewClient(vt.ClientConfig{})
	if err != nil {
		cleanup()
		return register.Service{}, nil, fmt.Errorf("create VT client: %w", err)
	}

	return register.Service{DB: db, VTClient: client}, cleanup, nil
}

// printAttemptResult writes a stable human-readable attempt summary.
func printAttemptResult(w io.Writer, result register.AttemptAddResult) {
	fmt.Fprintf(w, "watch_id: %d\n", result.Watch.ID)
	fmt.Fprintf(w, "outcome: %s\n", result.Outcome)
	fmt.Fprintf(w, "registered: %t\n", result.Registered)
	fmt.Fprintf(w, "section_status: %s\n", result.Evaluation.SectionStatus)
	fmt.Fprintf(w, "registration_window: %s\n", result.Evaluation.Window)
	fmt.Fprintf(w, "message: %s\n", result.Message)
}
