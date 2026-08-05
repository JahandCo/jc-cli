package templates

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.tmpl
var templateFS embed.FS

type ProjectTemplateData struct {
	Slug             string
	DisplayName      string
	ProjectId        string
	Visibility       string
	Scope            string
	Environment      string
	SdkVersion       string
	ProtocolVersion  string
	MechanicsVersion string
	PhaserVersion    string
}

// These packages don't move in lockstep -- interactive-protocol is at
// 0.4.0 (domains/user.ts gained listProjects, sdk-changelog.md's
// 2026-08-05 entry), interactive-sdk is at 0.6.0 (jc.user.listProjects(),
// same changelog entry -- and itself depends on protocol 0.4.0 exactly, so
// the two constants below have to move together), interactive-mechanics is
// at 0.3.0 (its own bump, for the new jc.db sugar layer -- createDb()),
// and phaser is a third-party dependency entirely (PHASER_INTEGRATION.md
// §3 -- pinned platform-wide the same way the SDK deps are, not a
// jahandco.config.json-managed version). Do not collapse these back into
// one shared constant without checking each package's actual published
// version first -- doing so previously would have generated package.json
// files with an unsatisfiable dependency range for mechanics.
//
// Bumping the *minor* digit here is deliberately the breaking gate: for a
// 0.x package, npm's own caret-range semantics treat the leftmost
// non-zero component as what "^" pins against, so "^0.5.0" does NOT admit
// 0.6.0. Keep these constants in sync with whatever's actually published,
// or freshly scaffolded titles silently keep installing the old version
// forever -- this has now happened three separate times (0.3.0 -> 0.4.0,
// 0.4.0 -> 0.5.0, and again 0.5.0 -> 0.6.0/protocol 0.3.0 -> 0.4.0, caught
// this time by an actual `npm install` reproduction against a freshly
// scaffolded title rather than by inspection). If you bump SdkVersion,
// check whatever `npm view @jahandco/interactive-sdk@<version>
// dependencies` says it needs and bump ProtocolVersion to match in the
// same change -- interactive-sdk 0.6.0 depends on protocol 0.4.0 as an
// exact pin (not a caret range), and protocol 0.4.0 briefly wasn't
// published to npm at all, which broke `npm install` for anyone resolving
// sdk to 0.6.0 by any means until it was published.
//
// No DefaultWorkerVersion here anymore -- @jahandco/platform-worker and its
// "start": "jahandco-worker" script were dropped from the scaffold
// (2026-08-04). Nothing in this CLI ever ran `npm start`, and the package
// was never the actual production execution path to begin with -- see
// game-sdk's runtime/README.md for why.
//
// No DefaultNextVersion here -- `jc init title` never actually adopted
// Next.js (see GetPackageJson/GetTitleComponent's own doc comments): the
// scaffold is a plain esbuild + vanilla-React SPA, unrelated to Next's own
// release cadence. A DefaultNextVersion constant existed here briefly
// (2026-08-05) as leftover from an abandoned Next.js migration -- it was
// never referenced by package.json.tmpl at all, so every scaffolded
// project silently ignored it. Removed rather than wired up, since nothing
// else in this CLI (dev.go's local sandbox, deploy.go's bundle packaging)
// assumes Next.js either.
const (
	DefaultSdkVersion       = "^0.6.0"
	DefaultProtocolVersion  = "^0.4.0"
	DefaultMechanicsVersion = "^0.3.0"
	DefaultPhaserVersion    = "^3.90.0"
)

func RenderTemplate(name string, data interface{}) (string, error) {
	tmpl, err := template.ParseFS(templateFS, name)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", name, err)
	}

	return buf.String(), nil
}

func GetPackageJson(slug string, sdkVersion string) (string, error) {
	if sdkVersion == "" {
		sdkVersion = DefaultSdkVersion
	}
	return RenderTemplate("package.json.tmpl", ProjectTemplateData{
		Slug:             slug,
		SdkVersion:       sdkVersion,
		ProtocolVersion:  DefaultProtocolVersion,
		MechanicsVersion: DefaultMechanicsVersion,
		PhaserVersion:    DefaultPhaserVersion,
	})
}

func GetTsconfig() (string, error) {
	return RenderTemplate("tsconfig.json.tmpl", nil)
}


func GetGitignore() (string, error) {
	return RenderTemplate("gitignore.tmpl", nil)
}

func GetIndexHtml(slug string) (string, error) {
	return RenderTemplate("index.html.tmpl", ProjectTemplateData{Slug: slug})
}

func GetClientTsx() (string, error) {
	return RenderTemplate("client.tsx.tmpl", nil)
}

func GetTitleComponent(slug, displayName string) (string, error) {
	return RenderTemplate("title.tsx.tmpl", ProjectTemplateData{Slug: slug, DisplayName: displayName})
}

func GetPhaserGameComponent() (string, error) {
	return RenderTemplate("phaser_game.tsx.tmpl", nil)
}

// GetPhaserScenes renders the Boot -> Preloader -> Game -> GameOver scene
// pipeline (modeled on phaserjs/template-nextjs's own scene/EventBus
// pattern) every title gets under src/scenes/. No "MainMenu" scene
// anymore -- the pre-game flow is the Lobby Structure (src/Title.tsx), a
// full React page above the canvas, not a Phaser scene; Preloader hands
// off straight to Game once it finishes.
func GetPhaserScenes(slug, displayName string) (map[string]string, error) {
	data := ProjectTemplateData{Slug: slug, DisplayName: displayName}

	sources := map[string]string{
		"Boot.ts":      "scenes_boot.ts.tmpl",
		"Preloader.ts": "scenes_preloader.ts.tmpl",
		"Game.ts":      "scenes_game.ts.tmpl",
		"GameOver.ts":  "scenes_game_over.ts.tmpl",
	}

	out := make(map[string]string, len(sources))
	for filename, tmplName := range sources {
		rendered, err := RenderTemplate(tmplName, data)
		if err != nil {
			return nil, err
		}
		out[filename] = rendered
	}
	return out, nil
}

// GetJahAndCoConfig renders jahandco.config.json, this project's only
// manifest -- jc.toml is gone (2026-08-04), and this file both drives
// local `jc dev`/`jc deploy` and ships inside the deploy bundle itself
// (developer-api's validateBundleArchive requires it). Deliberately no
// "gameId" field: a game id only exists once a title is deployed and
// published, so there's nothing real to write here at scaffold time.
func GetJahAndCoConfig(projectName, projectId, visibility, scope, environment string) (string, error) {
	return RenderTemplate("jahandco.config.json.tmpl", ProjectTemplateData{
		DisplayName: projectName,
		ProjectId:   projectId,
		Visibility:  visibility,
		Scope:       scope,
		Environment: environment,
	})
}

// GetRulesTemplate no longer selects or composes starter code from
// themeModel/features -- taxonomy is catalog/discovery metadata only now,
// set via the developer client against PATCH /v1/projects/{id}/metadata,
// not collected or persisted by `jc init title` at all anymore (removed
// 2026-07-26; see docs/TAXONOMY.md). It doesn't drive what gets scaffolded.
// @jahandco/interactive-mechanics' primitives (formerly appended per
// --features as illustrative snippets -- see that package's README, which
// now carries this same content) are always available to import directly
// instead. Rhythm is the one real exception: it runs on the separate
// Midnight Circuit-derived architecture, not yet extracted for this SDK --
// a genuinely different execution model, not a closed-taxonomy scaffold
// choice, so it keeps its own placeholder. `jc init title` itself has no
// way to request it anymore (there's no --theme flag left to pass
// "rhythm") -- this function's own rhythm branch is left as-is regardless,
// since Rhythm/Midnight Circuit is explicitly off-limits to touch until
// that extraction work actually starts, and some future caller may still
// have a real themeModel value to pass here.
func GetRulesTemplate(themeModel string) (string, error) {
	if themeModel == "rhythm" {
		return RenderTemplate("rules_rhythm.ts.tmpl", nil)
	}
	return RenderTemplate("rules_default.ts.tmpl", nil)
}
