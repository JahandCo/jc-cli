package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"jahandco/cli/internal/api"
	"jahandco/cli/internal/templates"
)

var (
	projName       string
	projVisibility string
	skipConfirm    bool

	linkName       string
	linkProject    string
	linkVisibility string
	linkRelink     bool
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects on the developer platform",
}

var projectCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new project",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := projName
		if len(args) > 0 {
			name = args[0]
		}
		if name == "" {
			return fmt.Errorf("project name is required: pass it as an argument or via --name flag")
		}

		vis := api.VisibilityPrivate
		if projVisibility == "public" {
			vis = api.VisibilityPublic
		}

		proj, err := api.CreateProject(name, vis)
		if err != nil {
			return err
		}

		fmt.Printf("[jc] created project %s (\"%s\", %s, unscoped)\n", proj.ID, proj.Name, proj.Visibility)
		fmt.Println("[jc] scope it later with: jc init title (select it when prompted)")
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		projects, err := api.ListProjects()
		if err != nil {
			return err
		}

		if len(projects) == 0 {
			fmt.Println("[jc] no projects yet -- create one with: jc project create \"<name>\"")
			return nil
		}

		for _, p := range projects {
			scope := "scope=unscoped"
			if p.Scope != nil && *p.Scope != "" {
				scope = "scope=" + *p.Scope
			}
			fmt.Printf("%s  %s  %s  visibility=%s\n", p.ID, p.Name, scope, p.Visibility)
		}
		return nil
	},
}

// findProjectByIDOrName resolves an id-or-name argument against an
// already-fetched project list -- prefers an exact ID match, then falls
// back to the first name match. ambiguous is true when more than one
// project shares that name, so callers can decide whether/how to warn.
func findProjectByIDOrName(projects []*api.Project, idOrName string) (target *api.Project, ambiguous bool, err error) {
	var matches []*api.Project
	for _, p := range projects {
		if p.ID == idOrName || p.Name == idOrName {
			matches = append(matches, p)
		}
	}

	if len(matches) == 0 {
		return nil, false, fmt.Errorf("[jc] no project matching \"%s\"", idOrName)
	}

	for _, p := range matches {
		if p.ID == idOrName {
			return p, false, nil
		}
	}
	return matches[0], len(matches) > 1, nil
}

var projectDeleteCmd = &cobra.Command{
	Use:   "delete [id-or-name]",
	Short: "Delete a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idOrName := args[0]
		projects, err := api.ListProjects()
		if err != nil {
			return err
		}

		target, ambiguous, err := findProjectByIDOrName(projects, idOrName)
		if err != nil {
			return err
		}
		if ambiguous {
			fmt.Printf("[jc] warning: multiple projects named \"%s\" -- deleting %s\n", idOrName, target.ID)
		}

		if !skipConfirm {
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("Delete project \"%s\" (%s)? This cannot be undone. [y/N]: ", target.Name, target.ID)
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

		if err := api.DeleteProject(target.ID); err != nil {
			return err
		}

		fmt.Printf("[jc] deleted project %s\n", target.ID)
		return nil
	},
}

// projectLinkCmd connects the current directory to a backend project --
// the counterpart to `jc init title --local` (and what `npx
// @jahandco/create` delegates to under the hood): scaffolding a title and
// linking it to a project used to be one inseparable step in `jc init
// title`, requiring `jc login` before a single local file existed. This
// command is that second step on its own, reusing the exact same
// resolveProject flow `jc init title` already uses (create new by --name,
// target an existing one by --project, or an interactive picker with
// neither) so "create a new project, link, scope" and "link to an
// existing project, scope" are both just this one command.
var projectLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Connect the current directory to a project, scoping it as a title",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		configPath := filepath.Join(cwd, "jahandco.config.json")
		if _, statErr := os.Stat(configPath); statErr == nil && !linkRelink {
			return fmt.Errorf("[jc] %s already exists -- this directory looks already linked. Pass --relink to overwrite it with a different project", configPath)
		}

		project, err := resolveProject(linkName, linkProject, linkVisibility)
		if err != nil {
			return err
		}

		scopeVal := ""
		if project.Scope != nil {
			scopeVal = *project.Scope
		}
		configJson, err := templates.GetJahAndCoConfig(project.Name, project.ID, string(project.Visibility), scopeVal, "development")
		if err != nil {
			return err
		}
		if err := os.WriteFile(configPath, []byte(configJson), 0644); err != nil {
			return err
		}

		fmt.Printf("[jc] linked %s to project %s (%s)\n", cwd, project.Name, project.ID)
		fmt.Println("[jc] next: npm run build && jc dev  (or jc deploy when ready)")
		return nil
	},
}

func init() {
	projectCreateCmd.Flags().StringVar(&projName, "name", "", "Name of the project")
	projectCreateCmd.Flags().StringVar(&projVisibility, "visibility", "private", "Visibility of the project (public or private)")

	projectDeleteCmd.Flags().BoolVar(&skipConfirm, "yes", false, "Skip confirmation prompt")
	projectDeleteCmd.Flags().BoolVar(&skipConfirm, "force", false, "Skip confirmation prompt (alias for --yes)")

	projectLinkCmd.Flags().StringVar(&linkName, "name", "", "Name of a new project to create (mutually exclusive with --project)")
	projectLinkCmd.Flags().StringVar(&linkProject, "project", "", "Id or name of an existing project to link to (scopes it first if not already scoped)")
	projectLinkCmd.Flags().StringVar(&linkVisibility, "visibility", "private", "Visibility of the project (public or private) -- only used with --name")
	projectLinkCmd.Flags().BoolVar(&linkRelink, "relink", false, "Overwrite an existing jahandco.config.json in this directory with a different project")

	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectLinkCmd)

	rootCmd.AddCommand(projectCmd)
}
