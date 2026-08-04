package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression test: StartRulesRunner's injected Node wrapper used to call
// handleIntent(envelope) with no second argument at all, so any rules.ts
// referencing context.roster (as both shipped templates and
// packages/mechanics's rules_card_board.ts.tmpl worked example do) crashed
// with "Cannot read properties of undefined (reading 'roster')" on its
// very first call, in every scaffolded title's local jc dev sandbox.
func TestStartRulesRunnerPassesContext(t *testing.T) {
	dir := t.TempDir()
	rulesPath := filepath.Join(dir, "rules.js")
	rulesJS := `
export function handleIntent(envelope, context) {
  return {
    roster: context.roster ?? null,
    hasAuth: typeof context.auth === "object",
    hasDb: typeof context.services?.db?.get === "function",
  };
}
`
	if err := os.WriteFile(rulesPath, []byte(rulesJS), 0644); err != nil {
		t.Fatal(err)
	}

	runner, err := StartRulesRunner(rulesPath)
	if err != nil {
		t.Fatalf("StartRulesRunner: %v", err)
	}
	defer runner.Close()

	envelope := `{"sdkVersion":"0.3.0","gameId":"g1","sessionToken":"dev-session","timestamp":1,"domain":"multiplayer","action":"submitIntent","payload":{}}`
	resp, err := runner.HandleIntent(envelope)
	if err != nil {
		t.Fatalf("HandleIntent: %v", err)
	}

	if strings.Contains(resp, "error") {
		t.Fatalf("handleIntent reported an error -- context likely wasn't passed at all: %s", resp)
	}
	if !strings.Contains(resp, `"roster":null`) {
		t.Fatalf("expected context.roster to be present-but-nullish (jc dev has no roster wiring), got: %s", resp)
	}
	if !strings.Contains(resp, `"hasAuth":true`) {
		t.Fatalf("expected context.auth to be an object, got: %s", resp)
	}
	if !strings.Contains(resp, `"hasDb":true`) {
		t.Fatalf("expected context.services.db.get to be a function, got: %s", resp)
	}
}

// Regression test: a rules bundle replying with multiplayer.StateView[]
// (stateViewsForPlayers' documented shape -- what every shipped
// multiplayer example, including rules_card_board.ts.tmpl, actually
// returns) used to go out over the dev-session tunnel completely
// unwrapped. Neither dev.go nor apps/client's DevSessionViewer.tsx do any
// per-recipient fan-out (that's apps/host's job in a real deployment, and
// jc dev has no apps/host in the loop) -- BridgeClient.multiplayer.
// submitIntent() is typed Promise<StateView> (a single object), so a real
// browser caller would get a bare array back: no crash, but
// view.publicState is always undefined and the title's UI never updates.
func TestUnwrapStateViewForCaller(t *testing.T) {
	multi := `{"sdkVersion":"0.3.0","gameId":"g1","sessionToken":"dev-session","timestamp":1,"domain":"multiplayer","action":"stateView","payload":[{"recipientPlayerId":"p1","revision":1,"publicState":{"whoseView":"p1"},"privateState":{}},{"recipientPlayerId":"p2","revision":1,"publicState":{"whoseView":"p2"},"privateState":{}}]}`

	out := unwrapStateViewForCaller("p2", []byte(multi))

	var envelope struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("unwrapped response isn't valid JSON: %v", err)
	}

	var view struct {
		RecipientPlayerID string `json:"recipientPlayerId"`
		PublicState       struct {
			WhoseView string `json:"whoseView"`
		} `json:"publicState"`
	}
	if err := json.Unmarshal(envelope.Payload, &view); err != nil {
		t.Fatalf("expected payload to unwrap to a single StateView object, still an array?: %v (payload: %s)", err, envelope.Payload)
	}
	if view.RecipientPlayerID != "p2" || view.PublicState.WhoseView != "p2" {
		t.Fatalf("expected p2's own StateView, got: %s", envelope.Payload)
	}
}

func TestUnwrapStateViewForCallerLeavesNonMultiplayerRepliesAlone(t *testing.T) {
	commerce := `{"sdkVersion":"0.3.0","gameId":"g1","sessionToken":"dev-session","timestamp":1,"domain":"commerce","action":"grantItem","payload":[1,2,3]}`
	out := unwrapStateViewForCaller("p1", []byte(commerce))
	if string(out) != commerce {
		t.Fatalf("commerce/leaderboard replies are title-owned and may legitimately be arrays -- must not be touched, got: %s", out)
	}
}

func TestUnwrapStateViewForCallerLeavesSingleObjectPayloadAlone(t *testing.T) {
	single := `{"sdkVersion":"0.3.0","gameId":"g1","sessionToken":"dev-session","timestamp":1,"domain":"multiplayer","action":"stateView","payload":{"recipientPlayerId":"p1","revision":1,"publicState":{},"privateState":{}}}`
	out := unwrapStateViewForCaller("p1", []byte(single))
	if string(out) != single {
		t.Fatalf("a title that already replies with a single StateView object shouldn't be rewritten, got: %s", out)
	}
}

func TestExtractPlayerID(t *testing.T) {
	envelope := `{"domain":"multiplayer","action":"submitIntent","payload":{"playerId":"p1","intent":"choose","data":{}}}`
	if got := extractPlayerID([]byte(envelope)); got != "p1" {
		t.Fatalf("expected \"p1\", got %q", got)
	}
	if got := extractPlayerID([]byte("not json")); got != "" {
		t.Fatalf("expected empty string for malformed JSON, got %q", got)
	}
}
