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
	SdkVersion       string
	ProtocolVersion  string
	MechanicsVersion string
	WorkerVersion    string
	PhaserVersion    string
	// WithUIExample selects the React + @jahandco/interactive-sdk UI-kit/
	// Lobby-Structure starter (client_ui_example.ts.tmpl/index_ui_example.html.tmpl)
	// over the default Phaser one, and adds react/react-dom + a tsx esbuild
	// loader for the client build. See GetClientTs/GetIndexHtml/GetPackageJson.
	// Orthogonal to Phaser: this toggles the *lobby* screen's UI kit, not
	// in-match rendering -- a title could reasonably want both, but that
	// composition isn't scaffolded yet (PHASER_INTEGRATION.md §6).
	WithUIExample bool
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
// jc.db sugar layer -- createDb()), platform-worker is still at 0.1.0, and
// phaser is a third-party dependency entirely (PHASER_INTEGRATION.md §3 --
// pinned platform-wide the same way the SDK deps are, not a jc.toml-managed
// version). Do not collapse these back into one shared constant without
// checking each package's actual published version first -- doing so
// previously would have generated package.json files with an unsatisfiable
// dependency range for mechanics/platform-worker.
//
// package.json.tmpl's "@jahandco/interactive-protocol" line used to read
// {{.SdkVersion}} too -- there was no ProtocolVersion field at all, so it
// silently tracked the SDK's constraint instead of its own. That's exactly
// the failure mode described above, just for protocol instead of mechanics:
// every fresh `jc init title` broke with an ETARGET npm install error the
// moment SdkVersion (0.4.0) and protocol's real published version (still
// 0.3.0) diverged.
const (
	DefaultSdkVersion       = "^0.4.0"
	DefaultProtocolVersion  = "^0.3.0"
	DefaultMechanicsVersion = "^0.3.0"
	DefaultWorkerVersion    = "^0.1.0"
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

func GetPackageJson(slug string, sdkVersion string, withUIExample bool) (string, error) {
	if sdkVersion == "" {
		sdkVersion = DefaultSdkVersion
	}
	return RenderTemplate("package.json.tmpl", ProjectTemplateData{
		Slug:             slug,
		SdkVersion:       sdkVersion,
		ProtocolVersion:  DefaultProtocolVersion,
		MechanicsVersion: DefaultMechanicsVersion,
		WorkerVersion:    DefaultWorkerVersion,
		PhaserVersion:    DefaultPhaserVersion,
		WithUIExample:    withUIExample,
	})
}

func GetTsconfig() (string, error) {
	return RenderTemplate("tsconfig.json.tmpl", nil)
}

// GetIndexHtml/GetClientTs: withUIExample=true scaffolds the React +
// interactive-sdk UI-kit/Lobby-Structure starter instead of the default
// bare-DOM one -- see docs/SDK_STRATEGY.md and packages/sdk-preview for the
// dev-only preview app this same UI kit is also viewable through.
func GetIndexHtml(displayName string, withUIExample bool) (string, error) {
	name := "index.html.tmpl"
	if withUIExample {
		name = "index_ui_example.html.tmpl"
	}
	return RenderTemplate(name, ProjectTemplateData{
		DisplayName: displayName,
	})
}

// GetClientTs's withUIExample source is a .tsx template (real JSX, not
// vanilla client.ts.tmpl) -- see GetClientFilename for the matching output
// filename callers must write it to.
func GetClientTs(slug string, withUIExample bool) (string, error) {
	name := "client.ts.tmpl"
	if withUIExample {
		name = "client_ui_example.tsx.tmpl"
	}
	return RenderTemplate(name, ProjectTemplateData{
		Slug: slug,
	})
}

// GetClientFilename is client.tsx for withUIExample (real JSX -- TypeScript
// only parses JSX syntax in a .tsx file, regardless of tsconfig's "jsx"
// option; esbuild is more lenient about this via --loader overrides, but
// `tsc --noEmit` -- what every scaffolded project's own "typecheck" script
// runs -- is not), client.ts otherwise.
func GetClientFilename(withUIExample bool) string {
	if withUIExample {
		return "client.tsx"
	}
	return "client.ts"
}

func GetJahAndCoConfig(slug string, projectId string) (string, error) {
	return RenderTemplate("jahandco.config.json.tmpl", ProjectTemplateData{
		Slug:      slug,
		ProjectId: projectId,
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
