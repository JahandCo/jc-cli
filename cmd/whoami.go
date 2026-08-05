package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"jahandco/cli/internal/api"
	"jahandco/cli/internal/auth"
)

// whoamiCmd reports local session state (credentials.json, expiry) and, if
// that looks valid, resolves it against developer-api's GET /v1/me (Clerk
// identity) and GET /v1/me/plan (billing plan) -- both double as a
// live check that developer-api still accepts the stored token, not just
// that it looks unexpired locally.
var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently logged-in session",
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := auth.LoadCredentials()
		if err != nil {
			return err
		}
		if creds == nil {
			return fmt.Errorf("[jc] not logged in -- run \"jc login\" first")
		}
		if auth.IsExpired(creds) {
			return fmt.Errorf("[jc] session expired -- run \"jc login\" again")
		}

		expiresAt := time.UnixMilli(creds.ObtainedAt + creds.ExpiresIn*1000)

		me, err := api.GetMe()
		if err != nil {
			fmt.Printf("[jc] warning: could not verify session with developer-api: %v\n", err)
			fmt.Printf("[jc] logged in (session expires %s)\n", expiresAt.Local().Format(time.RFC1123))
			return nil
		}
		fmt.Printf("[jc] logged in as %s (session expires %s)\n", me.Email, expiresAt.Local().Format(time.RFC1123))

		plan, err := api.GetMyPlan()
		if err != nil {
			fmt.Printf("[jc] warning: could not fetch plan: %v\n", err)
			return nil
		}
		fmt.Printf("[jc] plan: %s\n", plan.Slug)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
