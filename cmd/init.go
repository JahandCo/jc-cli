package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"jahandco/cli/internal/api"
	"jahandco/cli/internal/templates"
)

var (
	initName          string
	initProject       string
	initVisibility    string
	initWithUIExample bool
)

var nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)
var leadingTrailingDashRegex = regexp.MustCompile(`^-+|-+$`)

func slugify(name string) string {
	res := strings.ToLower(strings.TrimSpace(name))
	res = nonAlphaNumRegex.ReplaceAllString(res, "-")
	res = leadingTrailingDashRegex.ReplaceAllString(res, "")
	return res
}

// scopeIfNeeded scopes p as a title unless it's already scoped -- an
// already-scoped project is left untouched (its existing scope wins),
// so re-running init against it just re-scaffolds.
func scopeIfNeeded(p *api.Project) (*api.Project, error) {
	if p.Scope != nil && *p.Scope != "" {
		return p, nil
	}
	return api.SetProjectScope(p.ID, "title")
}

func resolveProject(nameArg string, projectArg string, visibilityArg string) (*api.Project, error) {
	if nameArg != "" && projectArg != "" {
		return nil, fmt.Errorf("[jc] --name and --project are mutually exclusive -- pass one or the other")
	}

	// Non-interactive: always create a brand new project.
	if nameArg != "" {
		vis := api.VisibilityPrivate
		if visibilityArg == "public" {
			vis = api.VisibilityPublic
		}
		created, err := api.CreateProject(nameArg, vis)
		if err != nil {
			return nil, err
		}
		return scopeIfNeeded(created)
	}

	// Non-interactive: target a specific existing project by id or name.
	if projectArg != "" {
		projects, err := api.ListProjects()
		if err != nil {
			return nil, err
		}
		target, ambiguous, err := findProjectByIDOrName(projects, projectArg)
		if err != nil {
			return nil, err
		}
		if ambiguous {
			fmt.Printf("[jc] warning: multiple projects named \"%s\" -- using %s\n", projectArg, target.ID)
		}
		return scopeIfNeeded(target)
	}

	// Interactive: pick any existing project (scoped or unscoped) or
	// create a new one. Scoped projects are included -- selecting one just
	// re-scaffolds it locally, it's not re-scoped.
	projects, err := api.ListProjects()
	if err != nil {
		return nil, err
	}

	options := make([]string, 0, len(projects)+1)
	for _, p := range projects {
		if p.Scope != nil && *p.Scope != "" {
			options = append(options, fmt.Sprintf("%s (already scoped)", p.Name))
		} else {
			options = append(options, p.Name)
		}
	}
	options = append(options, "+ Create new project")

	var selectedIndex int
	prompt := &survey.Select{
		Message: "Select a project to scaffold as a title, or create a new one:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selectedIndex); err != nil {
		return nil, err
	}

	if selectedIndex < len(projects) {
		return scopeIfNeeded(projects[selectedIndex])
	}

	var newName string
	namePrompt := &survey.Input{
		Message: "Project name:",
	}
	if err := survey.AskOne(namePrompt, &newName); err != nil {
		return nil, err
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}

	var newVisibility string
	visPrompt := &survey.Select{
		Message: "Visibility:",
		Options: []string{"private", "public"},
		Default: "private",
	}
	if err := survey.AskOne(visPrompt, &newVisibility); err != nil {
		return nil, err
	}

	vis := api.VisibilityPrivate
	if newVisibility == "public" {
		vis = api.VisibilityPublic
	}

	created, err := api.CreateProject(newName, vis)
	if err != nil {
		return nil, err
	}

	return scopeIfNeeded(created)
}

var initCmd = &cobra.Command{
	Use:   "init title [name]",
	Short: "Initialize a new game title",
	RunE: func(cmd *cobra.Command, args []string) error {
		var nameArg string
		if len(args) > 0 {
			nameArg = args[0]
		} else if initName != "" {
			nameArg = initName
		}

		project, err := resolveProject(nameArg, initProject, initVisibility)
		if err != nil {
			return err
		}

		slug := slugify(project.Name)
		if slug == "" {
			return fmt.Errorf("[jc] \"%s\" doesn't produce a valid project slug", project.Name)
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		projectDir := filepath.Join(cwd, slug)
		if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
			return fmt.Errorf("[jc] \"%s\" already exists in %s", slug, cwd)
		}

		// Create directories
		if err := os.MkdirAll(filepath.Join(projectDir, "src"), 0755); err != nil {
			return fmt.Errorf("[jc] failed to create project structure: %w", err)
		}

		// Generate package.json
		pkgJson, err := templates.GetPackageJson(slug, "", initWithUIExample)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(pkgJson), 0644); err != nil {
			return err
		}

		// Generate tsconfig.json
		tsconfig, err := templates.GetTsconfig()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "tsconfig.json"), []byte(tsconfig), 0644); err != nil {
			return err
		}

		// Generate index.html
		indexHtml, err := templates.GetIndexHtml(project.Name, initWithUIExample)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "index.html"), []byte(indexHtml), 0644); err != nil {
			return err
		}

		// Generate jahandco.config.json -- this project's only manifest (jc.toml
		// is gone). No gameId: that's only ever assigned once a title is
		// deployed and published, so there's nothing real to write yet.
		scopeVal := ""
		if project.Scope != nil {
			scopeVal = *project.Scope
		}
		configJson, err := templates.GetJahAndCoConfig(project.Name, project.ID, string(project.Visibility), scopeVal, "development")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "jahandco.config.json"), []byte(configJson), 0644); err != nil {
			return err
		}

		// Generate client.ts (or client.tsx for --with-ui-example -- see
		// GetClientFilename, tsc only parses JSX in a .tsx file)
		clientTs, err := templates.GetClientTs(slug, initWithUIExample)
		if err != nil {
			return err
		}
		clientFilename := templates.GetClientFilename(initWithUIExample)
		if err := os.WriteFile(filepath.Join(projectDir, "src", clientFilename), []byte(clientTs), 0644); err != nil {
			return err
		}

		// Generate rules.ts -- always the genre-agnostic default now that
		// `jc init title` no longer collects a Theme/Model at all (see
		// docs/TAXONOMY.md); Rhythm's own placeholder is still reachable via
		// GetRulesTemplate("rhythm") for any future caller, just not from
		// this CLI command anymore.
		rulesTs, err := templates.GetRulesTemplate("")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "src", "rules.ts"), []byte(rulesTs), 0644); err != nil {
			return err
		}

		// Taxonomy (Theme/Model, Genre, Features) is no longer collected or
		// persisted here at all -- it's purely developer-client-set metadata
		// now (PATCH /v1/projects/{id}/metadata), added after the fact from
		// apps/client, not part of scaffolding a title locally.
		fmt.Printf("[jc] scaffolded %s (project: %s)\n", slug, project.ID)
		fmt.Println("[jc] installing dependencies...")

		installCmd := exec.Command("npm", "install")
		installCmd.Dir = projectDir
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr

		if err := installCmd.Run(); err != nil {
			fmt.Printf("[jc] npm install failed -- run it manually from %s/\n", slug)
			return nil
		}

		fmt.Println("[jc] done. Next steps:")
		fmt.Printf("  cd %s && npm run build\n", slug)
		fmt.Printf("  jc dev %s\n", slug)

		return nil
	},
}

func init() {
	// Nested init command structure to support "jc init title [name]"
	// In Cobra, we can have "init" command, and add "title" as a subcommand.
	// But to match the CLI usage exactly, we can also make initCmd handle it directly,
	// or we can make "init" a command, and "title" a subcommand.
	// Let's do Cobra-native subcommands: "init" -> "title"
	initTitleCmd := &cobra.Command{
		Use:   "title [name]",
		Short: "Initialize a new game title",
		RunE:  initCmd.RunE,
	}

	initTitleCmd.Flags().StringVar(&initName, "name", "", "Name of a new project to create (mutually exclusive with --project)")
	initTitleCmd.Flags().StringVar(&initProject, "project", "", "Id or name of an existing project to scaffold (scopes it first if not already scoped)")
	initTitleCmd.Flags().StringVar(&initVisibility, "visibility", "private", "Visibility of the project (public or private) -- only used with --name")
	initTitleCmd.Flags().BoolVar(&initWithUIExample, "with-ui-example", false, "Scaffold src/client.ts using @jahandco/interactive-sdk's React UI kit + Lobby Structure (LobbyWrapper) instead of the default bare-DOM starter -- see packages/sdk-preview for a way to preview that same UI kit without a running platform backend")

	initCmd.AddCommand(initTitleCmd)
	rootCmd.AddCommand(initCmd)
}
