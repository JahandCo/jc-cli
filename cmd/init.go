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
	initName       string
	initProject    string
	initVisibility string
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

		// Create directories. app/ is a Next.js App Router tree (the Lobby
		// Structure + Phaser composition, see GetAppTitle/
		// GetPhaserGameComponent) -- src/ now holds only rules.ts, the one
		// file in this scaffold that isn't part of the Next.js app at all
		// (it's server-side isolate code, compiled by esbuild, same as
		// before). public/assets/ is Next.js's own static-assets
		// convention -- `next build` copies it into out/ verbatim, and jc
		// deploy zips all of out/ alongside dist/rules.js (see deploy.go's
		// createBundleArchive) -- created empty here so it exists for
		// Preloader.ts's this.load.setPath("assets") from the start, even
		// before a developer adds a real sprite/audio file.
		if err := os.MkdirAll(filepath.Join(projectDir, "src"), 0755); err != nil {
			return fmt.Errorf("[jc] failed to create project structure: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(projectDir, "app", "scenes"), 0755); err != nil {
			return fmt.Errorf("[jc] failed to create project structure: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(projectDir, "public", "assets"), 0755); err != nil {
			return fmt.Errorf("[jc] failed to create project structure: %w", err)
		}

		// Generate package.json
		pkgJson, err := templates.GetPackageJson(slug, "")
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

		// Generate next.config.js -- output: "export", since titles run
		// inside apps/host's sandboxed player iframe from a bare uploaded
		// bundle, not a Next.js server.
		nextConfig, err := templates.GetNextConfig()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "next.config.js"), []byte(nextConfig), 0644); err != nil {
			return err
		}

		// Generate .gitignore -- node_modules/.next/out/dist all need to
		// stay untracked; nothing wrote one of these before this scaffold
		// grew a real Next.js build pipeline (.next/, out/) to ignore.
		gitignore, err := templates.GetGitignore()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
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

		// Generate the Next.js App Router tree: layout.tsx (shell + <title>),
		// page.tsx (renders Title), Title.tsx (the Lobby Structure -> Phaser
		// handoff), PhaserGame.tsx (the ref-mounted canvas component every
		// title's Phaser.Game construction lives in now, instead of a flat
		// client.ts).
		appLayout, err := templates.GetAppLayout(project.Name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "app", "layout.tsx"), []byte(appLayout), 0644); err != nil {
			return err
		}

		appPage, err := templates.GetAppPage()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "app", "page.tsx"), []byte(appPage), 0644); err != nil {
			return err
		}

		appTitle, err := templates.GetAppTitle(slug, project.Name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "app", "Title.tsx"), []byte(appTitle), 0644); err != nil {
			return err
		}

		phaserGame, err := templates.GetPhaserGameComponent()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "app", "PhaserGame.tsx"), []byte(phaserGame), 0644); err != nil {
			return err
		}

		// Generate the Boot -> Preloader -> Game -> GameOver scene pipeline
		// PhaserGame.tsx imports from ./scenes/. No "MainMenu" scene --
		// the pre-game flow is Title.tsx's Lobby Structure now, a React
		// page above the canvas, not a Phaser scene.
		scenes, err := templates.GetPhaserScenes(slug, project.Name)
		if err != nil {
			return err
		}
		for filename, content := range scenes {
			if err := os.WriteFile(filepath.Join(projectDir, "app", "scenes", filename), []byte(content), 0644); err != nil {
				return err
			}
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

		// Two lines, not one "cd X && npm run build" -- printing that
		// chained form previously broke the *next* printed line ("jc dev
		// X") when both were pasted into the same shell: the cd's effect
		// persists past the first command, so "jc dev X" resolved to
		// X/X and failed with a confusing "config file not found" error.
		fmt.Println("[jc] done. Next steps:")
		fmt.Printf("  cd %s\n", slug)
		fmt.Println("  npm run dev      # iterate in a real browser (layout/styling only -- jc.* calls need jc dev's tunnel or a mocked bridge, see @jahandco/interactive-sdk's initJahAndCoMock)")
		fmt.Println("  npm run build    # compile for jc dev / jc deploy")
		fmt.Println("  jc dev")

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
	initTitleCmd.Flags().StringVar(&initVisibility, "visibility", "private", "Visibility of the project (public or private -- private requires a plan with the private_titles feature; the free/sandbox plan is public-only) -- only used with --name")

	initCmd.AddCommand(initTitleCmd)
	rootCmd.AddCommand(initCmd)
}
