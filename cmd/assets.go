package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"jahandco/cli/internal/api"
)

var assetsSkipConfirm bool

var assetsCmd = &cobra.Command{
	Use:   "assets",
	Short: "Manage a title's persistent asset library",
	Long: `Manage a title's persistent asset library -- images and audio uploaded
outside the "jc deploy" bundle flow, referenced at runtime via
jc.assets.get(assetId) in the Game SDK. Must be run from a title project
folder containing jahandco.config.json.`,
}

var assetsUploadCmd = &cobra.Command{
	Use:   "upload <path>",
	Short: "Upload a file into this title's asset library",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		manifest, err := readManifest(cwd)
		if err != nil {
			return err
		}

		path := args[0]
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("[jc] failed to read %s: %w", path, err)
		}

		filename := filepath.Base(path)
		fmt.Printf("[jc] uploading %s (%d bytes) to project %s...\n", filename, len(data), manifest.ProjectID)

		created, err := api.UploadAsset(manifest.ProjectID, filename, data)
		if err != nil {
			return err
		}

		fmt.Printf("[jc] uploaded -- asset id: %s\n", created.ID)
		fmt.Printf("[jc] reference it in your title code via: jc.assets.get(%q)\n", created.ID)
		return nil
	},
}

var assetsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List this title's uploaded assets",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		manifest, err := readManifest(cwd)
		if err != nil {
			return err
		}

		assets, err := api.ListAssets(manifest.ProjectID)
		if err != nil {
			return err
		}

		if len(assets) == 0 {
			fmt.Println("[jc] no assets yet -- upload one with: jc assets upload <path>")
			return nil
		}

		for _, a := range assets {
			fmt.Printf("%s  %s  %s  %d bytes  %s\n", a.ID, a.Filename, a.ContentType, a.SizeBytes, a.CreatedAt)
		}
		return nil
	},
}

var assetsRmCmd = &cobra.Command{
	Use:   "rm <assetId>",
	Short: "Delete an uploaded asset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		manifest, err := readManifest(cwd)
		if err != nil {
			return err
		}
		assetID := args[0]

		if !assetsSkipConfirm {
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("Delete asset %s? Any title code still calling jc.assets.get(%q) will start failing. [y/N]: ", assetID, assetID)
			choice, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			choice = strings.TrimSpace(strings.ToLower(choice))
			if choice != "y" && choice != "yes" {
				fmt.Println("[jc] cancelled")
				return nil
			}
		}

		if err := api.DeleteAsset(manifest.ProjectID, assetID); err != nil {
			return err
		}

		fmt.Printf("[jc] deleted asset %s\n", assetID)
		return nil
	},
}

func init() {
	assetsRmCmd.Flags().BoolVar(&assetsSkipConfirm, "yes", false, "Skip confirmation prompt")
	assetsRmCmd.Flags().BoolVar(&assetsSkipConfirm, "force", false, "Skip confirmation prompt (alias for --yes)")

	assetsCmd.AddCommand(assetsUploadCmd)
	assetsCmd.AddCommand(assetsListCmd)
	assetsCmd.AddCommand(assetsRmCmd)

	rootCmd.AddCommand(assetsCmd)
}
