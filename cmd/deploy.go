package cmd

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"jahandco/cli/internal/auth"
)

type ProjectManifest struct {
	Project struct {
		ID         string `toml:"id"`
		Name       string `toml:"name"`
		Scope      string `toml:"scope"`
		Visibility string `toml:"visibility"`
	} `toml:"project"`
	Deploy struct {
		Environment string `toml:"environment"`
	} `toml:"deploy"`
}

var deployEnv string

// requiredBundleFiles is what a title's compiled gameplay bundle must
// contain. dist/rules.js and dist/client.js come from the project's own
// "npm run build" (esbuild, see package.json.tmpl); index.html/
// jahandco.config.json are static. What actually executes dist/rules.js
// in production is game-studio's session-host service, not
// @jahandco/platform-worker (see game-sdk's runtime/README.md) -- that
// package is no longer part of the scaffold at all.
var requiredBundleFiles = []string{
	filepath.Join("dist", "rules.js"),
	filepath.Join("dist", "client.js"),
	"index.html",
	"jahandco.config.json",
}

// readManifest reads and parses jc.toml from cwd -- shared by deploy.go and
// bindings.go, both of which operate on "the title project in the current
// directory" rather than requiring an explicit project id/name argument.
func readManifest(cwd string) (*ProjectManifest, error) {
	manifestPath := filepath.Join(cwd, "jc.toml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("[jc] manifest file 'jc.toml' not found in current directory -- this command must be run from a title project folder")
	}

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read jc.toml: %w", err)
	}

	var manifest ProjectManifest
	if _, err := toml.Decode(string(manifestBytes), &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse jc.toml: %w", err)
	}
	if manifest.Project.ID == "" {
		return nil, fmt.Errorf("[jc] project ID is empty in jc.toml")
	}
	return &manifest, nil
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Build and deploy the current title's compiled gameplay bundle",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		manifest, err := readManifest(cwd)
		if err != nil {
			return err
		}
		projectID := manifest.Project.ID

		// Determine environment target
		env := "development"
		if deployEnv != "" {
			env = deployEnv
		} else if manifest.Deploy.Environment != "" {
			env = manifest.Deploy.Environment
		}

		env = strings.ToLower(strings.TrimSpace(env))
		if env == "dev" {
			env = "development"
		} else if env == "prod" {
			env = "production"
		}
		if env != "development" && env != "production" {
			return fmt.Errorf("[jc] invalid environment: must be 'development' ('dev') or 'production' ('prod') (got '%s')", env)
		}

		token, err := auth.GetToken()
		if err != nil {
			return err
		}

		developerAPIURL := os.Getenv("JAHANDCO_DEVELOPER_API_URL")
		if developerAPIURL == "" {
			developerAPIURL = "https://api-developer.jahandco.net"
		}

		// A Dockerfile is a harmless leftover from before "jc init" stopped
		// scaffolding one -- gameplay logic now ships to developer-api as a
		// compiled bundle, never as a docker build context, so there's
		// nothing left that reads this file. Warn, don't fail, since some
		// developers may still keep one around for their own local testing.
		if _, err := os.Stat(filepath.Join(cwd, "Dockerfile")); err == nil {
			fmt.Println("[jc] note: found a local Dockerfile -- it's no longer used by 'jc deploy' (gameplay logic ships as a compiled bundle now, not a container). Safe to remove.")
		}

		fmt.Println("[jc] building gameplay bundle (npm run build)...")
		buildCmd := exec.Command("npm", "run", "build")
		buildCmd.Dir = cwd
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("[jc] build failed: %w", err)
		}

		bundleBytes, err := createBundleArchive(cwd)
		if err != nil {
			return fmt.Errorf("[jc] failed to package bundle: %w", err)
		}

		fmt.Printf("[jc] uploading bundle for title \"%s\" (%s) to %s environment...\n", manifest.Project.Name, projectID, env)

		url := fmt.Sprintf("%s/v1/projects/%s/deploy?environment=%s", developerAPIURL, projectID, env)
		req, err := http.NewRequest("POST", url, bytes.NewReader(bundleBytes))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/zip")

		client := &http.Client{Timeout: 5 * time.Minute}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("[jc] deploy failed (%d): %s", resp.StatusCode, string(bodyBytes))
		}

		// Stream deploy logs from server in real-time. The server reports a
		// post-200 failure (provisioning error, no provisioner configured,
		// etc.) as a plain-text line within this already-200 stream, not as
		// a distinct HTTP status -- see deployBundle's doc comment. So a 200
		// status alone doesn't mean the deploy actually completed; only the
		// final "released ... deployed for project" line does.
		reader := bufio.NewReader(resp.Body)
		var lastLine string
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				fmt.Print(line)
				if trimmed := strings.TrimRight(line, "\n"); trimmed != "" {
					lastLine = trimmed
				}
			}
			if err != nil {
				if err == io.EOF {
					break
				}
				return fmt.Errorf("error reading deploy logs stream: %w", err)
			}
		}

		if !strings.Contains(lastLine, "deployed for project") {
			return fmt.Errorf("[jc] deploy did not complete successfully -- see log above (last line: %q)", lastLine)
		}

		fmt.Printf("[jc] Deployed successfully to %s\n", deployedURL(projectID, env))

		return nil
	},
}

// The CLI is the single source of truth for the URL it reports back to the
// developer -- the server used to print its own hardcoded copy of this same
// URL in the build-log stream, and the two drifted (one was updated to the
// path-based PTC pattern, the other still had the old wildcard-subdomain
// one) before this was consolidated into one place.
func deployedURL(projectID, env string) string {
	if env == "development" {
		return fmt.Sprintf("https://ptc.jahandco.dev/play/%s", projectID)
	}
	return fmt.Sprintf("https://play.jahandco.dev/play/%s", projectID)
}

// createBundleArchive packages just the compiled gameplay bundle -- not the
// project's whole source tree (that was the old Dockerfile-based deploy
// path's job). package.json is included when present purely so
// developer-api can read its "version" field the same way it always has;
// nothing in the runtime reads it.
func createBundleArchive(srcDir string) ([]byte, error) {
	for _, rel := range requiredBundleFiles {
		if _, err := os.Stat(filepath.Join(srcDir, rel)); err != nil {
			return nil, fmt.Errorf("missing %s -- did 'npm run build' succeed?", rel)
		}
	}

	files := append([]string{}, requiredBundleFiles...)
	if _, err := os.Stat(filepath.Join(srcDir, "package.json")); err == nil {
		files = append(files, "package.json")
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(srcDir, rel))
		if err != nil {
			return nil, err
		}
		w, err := zw.Create(rel)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func init() {
	deployCmd.Flags().StringVarP(&deployEnv, "environment", "e", "", "Target environment (development or production)")
	rootCmd.AddCommand(deployCmd)
}
