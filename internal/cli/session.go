package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/brennanhumphrey/seathawk/internal/session"
	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/vt"
	"github.com/spf13/cobra"
)

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage the captured VT session",
	}

	cmd.AddCommand(newSessionImportCmd())
	cmd.AddCommand(newSessionValidateCmd())
	cmd.AddCommand(newSessionShowCmd())
	return cmd
}

func newSessionImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <path|->",
		Short: "Import and validate captured VT credentials",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// The bookmarklet copies JSON; accepting "-" lets users paste through
			// stdin without writing credentials to an extra temporary file.
			payload, err := readCapturedCredentials(args[0], cmd.InOrStdin())
			if err != nil {
				return err
			}

			svc, cleanup, err := newSessionService(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			imported, err := svc.Import(cmd.Context(), payload)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "session imported and validated")
			printSessionSummary(cmd.OutOrStdout(), imported)
			return nil
		},
	}
	return cmd
}

func newSessionValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the stored VT session",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := newSessionService(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			validated, err := svc.ValidateCurrent(cmd.Context())
			printSessionSummary(cmd.OutOrStdout(), validated)
			return err
		},
	}
	return cmd
}

func newSessionShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show safe metadata for the stored VT session",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := newSessionService(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()

			current, err := svc.Current(cmd.Context())
			if err != nil {
				return err
			}

			printSessionSummary(cmd.OutOrStdout(), current)
			return nil
		},
	}
	return cmd
}

func newSessionService(ctx context.Context) (session.Service, func(), error) {
	_, db, cleanup, err := openAppDB(ctx)
	if err != nil {
		return session.Service{}, nil, err
	}

	client, err := vt.NewClient(vt.ClientConfig{})
	if err != nil {
		cleanup()
		return session.Service{}, nil, fmt.Errorf("create VT client: %w", err)
	}

	return session.Service{DB: db, VTClient: client}, cleanup, nil
}

func readCapturedCredentials(path string, stdin io.Reader) (session.CapturedCredentials, error) {
	var reader io.Reader
	if path == "-" {
		reader = stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return session.CapturedCredentials{}, fmt.Errorf("open import file: %w", err)
		}
		defer file.Close()
		reader = file
	}

	var payload session.CapturedCredentials
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		return session.CapturedCredentials{}, fmt.Errorf("parse import JSON: %w", err)
	}
	return payload, nil
}

func printSessionSummary(w io.Writer, current store.Session) {
	if current.ID == 0 {
		fmt.Fprintln(w, "session: unavailable")
		return
	}

	fmt.Fprintf(w, "status: %s\n", current.Status)
	fmt.Fprintf(w, "captured_at: %s\n", current.CapturedAt.UTC().Format("2006-01-02T15:04:05Z07:00"))
	if current.LastValidatedAt.Valid {
		fmt.Fprintf(w, "last_validated_at: %s\n", current.LastValidatedAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00"))
	} else {
		fmt.Fprintln(w, "last_validated_at: never")
	}
	// pers_id is less sensitive than authtoken/pers_id_proof but still an opaque
	// credential-adjacent identifier, so only show enough to help the user
	// recognize which browser session was imported.
	fmt.Fprintf(w, "pers_id: %s\n", maskSecret(current.PersID))
}

func maskSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return strings.Repeat("*", len(value))
	}
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}
