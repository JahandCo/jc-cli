package cmd

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"jahandco/cli/internal/auth"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to the Jah and Co Developer Platform",
	RunE: func(cmd *cobra.Command, args []string) error {
		verifier, challenge, state, err := auth.GeneratePKCE()
		if err != nil {
			return fmt.Errorf("failed to generate PKCE tokens: %w", err)
		}

		u, err := url.Parse(auth.ClerkAuthorizeURL)
		if err != nil {
			return err
		}

		q := u.Query()
		q.Set("response_type", "code")
		q.Set("client_id", auth.ClerkClientID)
		q.Set("redirect_uri", auth.ClerkRedirectURI)
		q.Set("scope", auth.ClerkScopes)
		q.Set("state", state)
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
		u.RawQuery = q.Encode()

		fmt.Printf("[jc] To log in, open this URL in any browser:\n\n  %s\n\n", u.String())

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Paste the code shown after you finish logging in: ")
		code, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		code = strings.TrimSpace(code)

		if code == "" {
			return fmt.Errorf("[jc] login cancelled -- no code entered")
		}

		fmt.Println("[jc] exchanging authorization code...")
		creds, err := auth.ExchangeCode(code, verifier)
		if err != nil {
			return fmt.Errorf("[jc] login failed: %w", err)
		}

		if err := auth.SaveCredentials(creds); err != nil {
			return fmt.Errorf("[jc] failed to save credentials: %w", err)
		}

		fmt.Println("[jc] logged in.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
