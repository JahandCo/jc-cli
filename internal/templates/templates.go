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
	NextVersion      string
}

// These packages don't move in lockstep -- interactive-protocol is at
// 0.3.0 (bumped for the envelope-level BridgeError fix, GAPS.md's entry on
// it), interactive-sdk is at 0.4.0 (ReadyToggle, sdk-changelog.md's entry
// on it -- bumping the *minor* digit here is deliberately the breaking
// gate: for a 0.x package, npm's own caret-range semantics treat the
// leftmost non-zero component as what "^" pins against, so "^0.3.0" does
// NOT admit 0.4.0. Keep this constant in sync with whatever's actually
// published, or freshly scaffolded titles silently keep installing the old
// version forever -- exactly what happened here before this comment was
// written: rps-showdown's own real `npm install` kept resolving 0.3.1 even
// after 0.4.0 was live on npm, because DefaultSdkVersion was still
// "^0.3.0"), interactive-mechanics is at 0.3.0 (its own bump, for the new
// jc.db sugar layer -- createDb()), and phaser is a third-party dependency
// entirely (PHASER_INTEGRATION.md §3 -- pinned platform-wide the same way
// the SDK deps are, not a jahandco.config.json-managed version). Do not
// collapse these back into one shared constant without checking each
// package's actual published version first -- doing so previously would
// have generated package.json files with an unsatisfiable dependency
// range for mechanics.
//
// package.json.tmpl's "@jahandco/interactive-protocol" line used to read
// {{.SdkVersion}} too -- there was no ProtocolVersion field at all, so it
// silently tracked the SDK's constraint instead of its own. That's exactly
// the failure mode described above, just for protocol instead of mechanics:
// every fresh `jc init title` broke with an ETARGET npm install error the
// moment SdkVersion (0.4.0) and protocol's real published version (still
// 0.3.0) diverged.
//
// No DefaultWorkerVersion here anymore -- @jahandco/platform-worker and its
// "start": "jahandco-worker" script were dropped from the scaffold
// (2026-08-04). Nothing in this CLI ever ran `npm start`, and the package
// was never the actual production execution path to begin with -- see
// game-sdk's runtime/README.md for why.
// DefaultSdkVersion bumped to ^0.5.0 for the new
// @jahandco/interactive-sdk/phaser-kit subpath (BaseGameScene/EventBus,
// sdk-changelog.md's 2026-08-05 entry) -- GetPhaserScenes' Game.ts.tmpl
// imports it, so a title scaffolded against an older SDK version would
// fail to resolve that import. See the DefaultMechanicsVersion comment
// above for why this constant isn't collapsed with the others.
//
// DefaultNextVersion is new (2026-08-05, this same scaffold rewrite):
// `jc init title` now generates a real Next.js app instead of a bare
// esbuild + vanilla-DOM one -- see GetAppTitle/GetPhaserGameComponent's
// own doc comments for why (the short version: no starter used to compose
// the Lobby Structure and Phaser gameplay together at all, and the SDK's
// own EventBus/CURRENT_SCENE_READY convention was already modeled on
// phaserjs/template-nextjs without this CLI ever actually adopting that
// template). Not version-locked to the SDK/protocol/mechanics packages
// above -- Next.js's own release cadence is unrelated to theirs.
const (
	DefaultSdkVersion       = "^0.5.0"
	DefaultProtocolVersion  = "^0.3.0"
	DefaultMechanicsVersion = "^0.3.0"
	DefaultPhaserVersion    = "^3.90.0"
	DefaultNextVersion      = "^15.0.0"
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
		NextVersion:      DefaultNextVersion,
	})
}

func GetTsconfig() (string, error) {
	return RenderTemplate("tsconfig.json.tmpl", nil)
}

func GetNextConfig() (string, error) {
	return RenderTemplate("next.config.js.tmpl", nil)
}

func GetGitignore() (string, error) {
	return RenderTemplate("gitignore.tmpl", nil)
}

// GetAppLayout/GetAppPage/GetAppTitle/GetPhaserGameComponent together
// replace the old GetIndexHtml/GetClientTs split (client.ts.tmpl vs.
// client_ui_example.tsx.tmpl, toggled by a --with-ui-example flag that no
// longer exists). That split never generated a title with *both* the Lobby
// Structure and real Phaser gameplay -- see the removed ProjectTemplateData
// doc comment that used to admit as much ("a title could reasonably want
// both, but that composition isn't scaffolded yet"). Every title now gets
// both, composed the way phaserjs/template-nextjs's own reference
// architecture does it: a Next.js app, a <PhaserGame/> client component
// that mounts Phaser.Game into a ref'd div via useLayoutEffect, and
// EventBus/CURRENT_SCENE_READY (already exported by
// @jahandco/interactive-sdk/phaser-kit, previously unused by this CLI's
// own scaffold) as the one-way channel Phaser uses to tell React a scene
// is ready.
func GetAppLayout(displayName string) (string, error) {
	return RenderTemplate("app_layout.tsx.tmpl", ProjectTemplateData{DisplayName: displayName})
}

func GetAppPage() (string, error) {
	return RenderTemplate("app_page.tsx.tmpl", nil)
}

func GetAppTitle(slug, displayName string) (string, error) {
	return RenderTemplate("app_title.tsx.tmpl", ProjectTemplateData{Slug: slug, DisplayName: displayName})
}

func GetPhaserGameComponent() (string, error) {
	return RenderTemplate("app_phaser_game.tsx.tmpl", nil)
}

// GetPhaserScenes renders the Boot -> Preloader -> Game -> GameOver scene
// pipeline (modeled on phaserjs/template-nextjs's own scene/EventBus
// pattern) every title gets under app/scenes/. No "MainMenu" scene
// anymore -- the pre-game flow is the Lobby Structure (app/Title.tsx), a
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
