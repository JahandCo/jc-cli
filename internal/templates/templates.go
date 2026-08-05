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
	Slug            string
	DisplayName     string
	ProjectId       string
	Visibility      string
	Scope           string
	Environment     string
	SdkVersion      string
	ProtocolVersion string
	PhaserVersion   string
}

// @jahandco/game-sdk (the IoC rewrite of the old @jahandco/interactive-sdk
// -- see game-sdk's own sdk-changelog.md 2026-08-05 entry for the full
// history of that split) is now what jc init title scaffolds against, at
// 1.0.0 -- a fresh package identity, not a semver-compatible continuation
// of interactive-sdk. @jahandco/interactive-protocol is unchanged by that
// rewrite and stays on its own 0.4.0 line. @jahandco/interactive-mechanics
// is gone -- retired, not ported (see game-sdk's changelog) -- there is no
// MechanicsVersion constant anymore, and freshly scaffolded titles don't
// depend on it at all. phaser is a third-party dependency entirely
// (PHASER_INTEGRATION.md §3 -- pinned platform-wide the same way the SDK
// deps are, not a jahandco.config.json-managed version).
//
// Bumping the *minor* digit here is deliberately the breaking gate: for a
// 0.x package, npm's own caret-range semantics treat the leftmost
// non-zero component as what "^" pins against, so "^0.5.0" does NOT admit
// 0.6.0 -- game-sdk is 1.x now, so a caret range there instead admits any
// 1.x, and only a bump past 2.0.0 would need this same care again. Keep
// these constants in sync with whatever's actually published, or freshly
// scaffolded titles silently keep installing the old version forever --
// check whatever `npm view @jahandco/game-sdk@<version> dependencies`
// says it needs and bump ProtocolVersion to match in the same change if
// game-sdk ever pins protocol to an exact version again.
//
// No DefaultWorkerVersion here anymore -- @jahandco/platform-worker and its
// "start": "jahandco-worker" script were dropped from the scaffold
// (2026-08-04), and the runtime package itself was deleted outright in the
// same game-sdk rewrite that produced @jahandco/game-sdk. Nothing in this
// CLI ever ran `npm start`, and the package was never the actual
// production execution path to begin with -- see game-studio's
// session-host instead.
const (
	DefaultSdkVersion      = "^1.0.0"
	DefaultProtocolVersion = "^0.4.0"
	DefaultPhaserVersion   = "^3.90.0"
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
		Slug:            slug,
		SdkVersion:      sdkVersion,
		ProtocolVersion: DefaultProtocolVersion,
		PhaserVersion:   DefaultPhaserVersion,
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

// GetClientTs renders src/client.ts -- the entire entrypoint a title's own
// code needs: Engine.launch({...}) with its scene list. No React, no
// hand-built Phaser.Game config, no manual bridge/plugin wiring -- Engine
// (see @jahandco/game-sdk) owns the boot sequence, including the default
// lobby, itself.
func GetClientTs(slug, displayName string) (string, error) {
	return RenderTemplate("client.ts.tmpl", ProjectTemplateData{Slug: slug, DisplayName: displayName})
}

// GetWebpackClientConfig/GetWebpackRulesConfig render the two build
// configs every scaffolded title gets -- webpack replaced esbuild here;
// see each .tmpl's own doc comment for why they're two separate configs
// (the client bundle targets a browser, dist/rules.js targets Node and
// must stay a single self-contained module for session-host's V8
// isolates, which have zero runtime module resolution).
func GetWebpackClientConfig() (string, error) {
	return RenderTemplate("webpack.client.js.tmpl", nil)
}

func GetWebpackRulesConfig() (string, error) {
	return RenderTemplate("webpack.rules.js.tmpl", nil)
}

// GetPhaserScenes renders the Preloader -> Game -> GameOver scene pipeline
// every title gets under src/scenes/. No "Boot" scene here anymore --
// Engine runs its own internal boot scene ahead of Preloader now, so
// there's nothing title-specific left in what Boot used to do. No
// "MainMenu" scene either -- Engine's default lobby (LobbyClient) runs
// between Preloader and Game instead of a title-authored menu scene.
func GetPhaserScenes(slug, displayName string) (map[string]string, error) {
	data := ProjectTemplateData{Slug: slug, DisplayName: displayName}

	sources := map[string]string{
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
// Rhythm is the one real exception: it runs on the separate Midnight
// Circuit-derived architecture, not yet extracted for this SDK -- a
// genuinely different execution model, not a closed-taxonomy scaffold
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
