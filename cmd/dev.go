package cmd

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"jahandco/cli/internal/api"
	"jahandco/cli/internal/auth"
)

var (
	playAPIURL      string
	runnerMutex     sync.Mutex
	platformAPIWarn sync.Once
)

// playAPIDomains mirrors apps/host's PLAY_API_DOMAINS -- only the
// domains play-api still owns centrally (identity, lobby coordination,
// the ai placeholder). commerce/leaderboard moved to being title-owned (see
// play-api's internal/bridge -- it no longer implements them at all),
// so in production those route through dispatch to the title's own worker;
// jc dev has no worker pod to dispatch to, so they fall through to the
// local rules bundle here instead, same as multiplayer.* always has.
var playAPIDomains = map[string]bool{
	"lobby": true,
	"user":  true,
	"ai":    true,
}

// forwardToPlatformAPI POSTs the raw envelope bytes as-is -- every scaffolded
// jahandco.config.json ships with the "dev-session" sentinel sessionToken,
// which only a play-api instance started with PLAY_API_DEV_MODE=true
// will accept (see internal/auth.DevBypassValidator). A real deployed
// play-api rejects it like any other invalid token, which surfaces here
// as a plain error -- the caller falls back to the local rules bundle.
func forwardToPlatformAPI(msg []byte) ([]byte, error) {
	resp, err := http.Post(playAPIURL+"/v1/bridge", "application/json", bytes.NewReader(msg))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("play-api returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// warnPlatformAPIUnreachableOnce avoids spamming the console on every single
// message once it's clear play-api isn't reachable (or isn't running in
// dev mode) for this sandbox session.
func warnPlatformAPIUnreachableOnce(err error) {
	platformAPIWarn.Do(func() {
		fmt.Printf("[sandbox] warning: play-api unreachable at %s (%v) -- falling back to the local rules bundle for lobby/user/ai. Run play-api locally with PLAY_API_DEV_MODE=true to test against the real thing.\n", playAPIURL, err)
	})
}

type RulesRunner struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	scanner *bufio.Scanner
}

func StartRulesRunner(rulesPath string) (*RulesRunner, error) {
	jsCode := fmt.Sprintf(`
import { handleIntent } from 'file://%s';
import readline from 'readline';

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  terminal: false
});

// Mirrors @jahandco/interactive-protocol's WorkerContext shape (see
// context.ts): auth present but userId absent (no real session-token
// validation happens in this local runner), roster absent (jc dev has no
// dispatch/roster wiring -- rules.ts templates fall back to their own
// DEFAULT_PLAYERS via "context.roster ?? DEFAULT_PLAYERS", which only
// works if context.roster is undefined rather than context itself being
// undefined), services.db rejecting with a clear error instead of a bare
// TypeError, since db mediation only exists in the real session host this
// local runner doesn't replicate.
function makeContext() {
  return {
    auth: {},
    services: {
      db: {
        get: () => Promise.reject(new Error("[jc dev] context.services.db is not available in the local sandbox -- db mediation is provided by the real session host, not jc dev's local rules runner")),
        set: () => Promise.reject(new Error("[jc dev] context.services.db is not available in the local sandbox -- db mediation is provided by the real session host, not jc dev's local rules runner")),
      },
    },
  };
}

rl.on('line', (line) => {
  try {
    const envelope = JSON.parse(line);
    const response = handleIntent(envelope, makeContext());
    if (response) {
      console.log(JSON.stringify(response));
    } else {
      console.log(JSON.stringify({ __empty: true }));
    }
  } catch (e) {
    console.error("ERROR:", e);
    console.log(JSON.stringify({ __empty: true, error: e.message }));
  }
});
`, filepath.ToSlash(rulesPath)) // Ensure slashes are correct for import URLs

	cmd := exec.Command("node", "--input-type=module", "--eval", jsCode)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &RulesRunner{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		scanner: bufio.NewScanner(stdout),
	}, nil
}

func (r *RulesRunner) HandleIntent(envelopeJSON string) (string, error) {
	_, err := io.WriteString(r.stdin, envelopeJSON+"\n")
	if err != nil {
		return "", err
	}

	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}

	return r.scanner.Text(), nil
}

func (r *RulesRunner) Close() {
	r.stdin.Close()
	_ = r.cmd.Process.Kill()
}

// DomainMockRunner resolves lobby/user/ai locally when no play-api is
// reachable, instead of falling through to the developer's own rules.ts
// (which was never meant to implement those -- they're platform-owned in
// production). It wraps @jahandco/interactive-sdk's own MockBridgeClient
// -- already a dependency of every scaffolded project -- run in a small
// Node subprocess, so the exact same mock lobby/user/ai behavior
// sdk-preview already gives developers in the browser is available here
// too, with zero duplicated logic.
//
// Unlike RulesRunner, this isn't a strict one-request-one-reply pairing:
// a single incoming call (e.g. "kickPlayer") can produce a direct reply
// *and* trigger an unprompted lobbyState push to every other listener,
// exactly like a real deployment's dual reply+broadcast channel (see
// MockBridgeClient's own doc comment). So every stdout line -- reply or
// push -- is just forwarded to whatever the current session's onMessage
// callback is, asynchronously, rather than read back synchronously per
// call.
type DomainMockRunner struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser

	mu        sync.Mutex
	onMessage func([]byte)
}

// domainMockScript maps each lobby/user/ai wire action (see bridgeClient.ts
// -- these are the *wire* names, not MockBridgeClient's own method names,
// which differ for several of them: create->createLobby, kick->kickPlayer,
// etc.) onto the matching MockBridgeClient call, and wires its onLobbyState/
// onChatMessage listeners to emit unprompted push envelopes the same way a
// real deployment's lobby broadcast would.
const domainMockScript = `
import { MockBridgeClient } from '@jahandco/interactive-sdk';
import readline from 'readline';

const mock = new MockBridgeClient({
  gameId: %q,
  otherMembers: [
    { userId: 'mock-user-2', username: 'Alex' },
    { userId: 'mock-user-3', username: 'Sam' },
  ],
});

const actionToMethod = {
  lobby: {
    createLobby: (req) => mock.lobby.create(req),
    joinLobby: (req) => mock.lobby.join(req),
    leaveLobby: (req) => mock.lobby.leave(req),
    setReady: (req) => mock.lobby.setReady(req),
    kickPlayer: (req) => mock.lobby.kick(req),
    mutePlayer: (req) => mock.lobby.mute(req),
    unmutePlayer: (req) => mock.lobby.unmute(req),
    startGame: (req) => mock.lobby.start(req),
    sendMessage: (req) => mock.lobby.sendMessage(req),
  },
  user: {
    getProfile: () => mock.user.getProfile(),
  },
  ai: {
    requestJudgment: (req) => mock.ai.requestJudgment(req),
    generateContent: (req) => mock.ai.generateContent(req),
  },
};

function emit(envelope) {
  process.stdout.write(JSON.stringify(envelope) + '\n');
}

// Unprompted pushes -- no requestId, dispatched client-side purely by
// domain+action (see BridgeClient.on/handleMessage). Fires on every lobby
// mutation after the initial create/join (mirrors MockBridgeClient's own
// pushLobby() calls).
mock.lobby.onLobbyState((state) => {
  emit({ sdkVersion: '0.3.0', gameId: %q, sessionToken: 'dev-session', timestamp: Date.now(), domain: 'lobby', action: 'lobbyState', payload: state });
});
mock.lobby.onChatMessage((message) => {
  emit({ sdkVersion: '0.3.0', gameId: %q, sessionToken: 'dev-session', timestamp: Date.now(), domain: 'lobby', action: 'chatMessage', payload: message });
});

const rl = readline.createInterface({ input: process.stdin, output: process.stdout, terminal: false });

rl.on('line', async (line) => {
  let envelope;
  try {
    envelope = JSON.parse(line);
  } catch {
    return;
  }

  const handler = actionToMethod[envelope.domain] && actionToMethod[envelope.domain][envelope.action];
  if (typeof handler !== 'function') {
    emit({ ...envelope, payload: { type: 'error', code: 'unimplemented', message: 'no mock handler for ' + envelope.domain + '.' + envelope.action } });
    return;
  }

  try {
    const result = await handler(envelope.payload);
    emit({ ...envelope, timestamp: Date.now(), payload: result });
  } catch (e) {
    emit({ ...envelope, payload: { type: 'error', code: 'mock-error', message: e.message } });
  }
});
`

// StartDomainMockRunner spawns the mock responder with projectDir as its
// cwd, so the bare "@jahandco/interactive-sdk" import resolves against
// that project's own node_modules (no bundling/path juggling needed --
// it's already a real dependency every scaffolded project installs).
func StartDomainMockRunner(projectDir, gameID string) (*DomainMockRunner, error) {
	jsCode := fmt.Sprintf(domainMockScript, gameID, gameID, gameID)

	cmd := exec.Command("node", "--input-type=module", "--eval", jsCode)
	cmd.Dir = projectDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	r := &DomainMockRunner{cmd: cmd, stdin: stdin}
	go r.readLoop(stdout)
	return r, nil
}

func (r *DomainMockRunner) readLoop(stdout io.ReadCloser) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		r.mu.Lock()
		cb := r.onMessage
		r.mu.Unlock()
		if cb == nil {
			continue
		}
		msg := make([]byte, len(line))
		copy(msg, line)
		cb(msg)
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("[sandbox] mock lobby/user/ai responder exited: %v\n", err)
	}
}

// SetOnMessage points every subsequent reply/push at cb -- runSession calls
// this once per host connection (bound to that connection's own writeFrame),
// clearing it back to nil when the session ends, so a stale reconnect never
// writes into a closed websocket.
func (r *DomainMockRunner) SetOnMessage(cb func([]byte)) {
	r.mu.Lock()
	r.onMessage = cb
	r.mu.Unlock()
}

// Send is fire-and-forget -- correlation happens purely via the envelope's
// own requestId once the reply comes back through onMessage, same as a
// real deployment's client-side BridgeClient already assumes.
func (r *DomainMockRunner) Send(msg []byte) error {
	_, err := r.stdin.Write(append(msg, '\n'))
	return err
}

func (r *DomainMockRunner) Close() {
	r.stdin.Close()
	_ = r.cmd.Process.Kill()
}

// devSessionFrame is the wire shape used on the dev-session host connection
// to developer-api -- see that service's internal/devsession/manager.go,
// which this must stay byte-for-byte compatible with. "bridge" frames carry
// the same envelope JSON the rules runner and play-api have always spoken;
// "asset_request"/"asset" let the project console's Dev tab read this
// project's compiled output through the tunnel instead of a local port.
type devSessionFrame struct {
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	RequestID   string          `json:"requestId,omitempty"`
	Path        string          `json:"path,omitempty"`
	Status      int             `json:"status,omitempty"`
	ContentType string          `json:"contentType,omitempty"`
	Body        string          `json:"body,omitempty"` // base64, asset response only
	Error       string          `json:"error,omitempty"`
}

// processEnvelope hands a gameplay-domain envelope (multiplayer/commerce/
// leaderboard -- anything not in playAPIDomains) to the developer's own
// local rules bundle. Returns nil, nil for envelopes that produce no reply
// (mirrors the old handler's "__empty" check). lobby/user/ai never reach
// this -- see handleBridgeFrame, which routes those to play-api or the
// mock responder instead.
func processEnvelope(runner *RulesRunner, msg []byte) ([]byte, error) {
	runnerMutex.Lock()
	resp, err := runner.HandleIntent(string(msg))
	runnerMutex.Unlock()
	if err != nil {
		return nil, err
	}
	if resp == "" || strings.Contains(resp, `"__empty":true`) {
		return nil, nil
	}
	return []byte(resp), nil
}

// serveAsset reads a single file out of root (the project directory) for a
// developer-api asset_request frame. reqPath comes from the console
// iframe's request path by way of developer-api -- filepath.Clean plus the
// root-prefix check keep it from ever escaping the project directory.
func serveAsset(root, requestID, reqPath string) devSessionFrame {
	clean := filepath.Clean(string(filepath.Separator) + reqPath)
	full := filepath.Join(root, clean)
	if full != filepath.Clean(root) && !strings.HasPrefix(full, filepath.Clean(root)+string(filepath.Separator)) {
		return devSessionFrame{Type: "asset", RequestID: requestID, Error: "invalid path"}
	}

	data, err := os.ReadFile(full)
	if err != nil {
		return devSessionFrame{Type: "asset", RequestID: requestID, Status: http.StatusNotFound, Error: err.Error()}
	}

	contentType := mime.TypeByExtension(filepath.Ext(full))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	return devSessionFrame{
		Type:        "asset",
		RequestID:   requestID,
		Status:      http.StatusOK,
		ContentType: contentType,
		Body:        base64.StdEncoding.EncodeToString(data),
	}
}

// hostWebSocketURL turns developer-api's own base URL (http(s)) into the
// dev-session host-ws URL (ws(s)) for this project.
func hostWebSocketURL(projectID string) (string, error) {
	base := api.DeveloperAPIURL()
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid developer-api URL %q: %w", base, err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported developer-api URL scheme %q", u.Scheme)
	}
	u.Path = fmt.Sprintf("/v1/projects/%s/dev-session/host-ws", projectID)
	return u.String(), nil
}

// dialHost opens the dev-session host connection, authenticated with the
// same CLI OAuth token every other jc command already uses (see
// internal/api's request[T]).
func dialHost(projectID string) (*websocket.Conn, error) {
	wsURL, err := hostWebSocketURL(projectID)
	if err != nil {
		return nil, err
	}

	token, err := auth.GetToken()
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("developer-api rejected the dev session (%d): %s", resp.StatusCode, string(body))
		}
		return nil, err
	}
	return conn, nil
}

// handleBridgeFrame is the routing decision for one incoming bridge
// envelope: lobby/user/ai go to play-api if reachable, else the built-in
// mock responder (never to the developer's own rules.ts, which was never
// meant to implement those); everything else goes straight to the local
// rules bundle. mockRunner's replies/pushes arrive asynchronously via its
// own onMessage callback (wired once per session in runSession), not as a
// return value here -- see DomainMockRunner's doc comment for why.
func handleBridgeFrame(payload json.RawMessage, runner *RulesRunner, mockRunner *DomainMockRunner, writeFrame func(devSessionFrame)) {
	fmt.Printf("[sandbox] <- %s\n", string(payload))

	var probe struct {
		Domain string `json:"domain"`
	}
	_ = json.Unmarshal(payload, &probe)

	if playAPIDomains[probe.Domain] {
		if playAPIURL != "" {
			if body, fwdErr := forwardToPlatformAPI(payload); fwdErr == nil {
				fmt.Printf("[sandbox] -> %s (via play-api)\n", string(body))
				writeFrame(devSessionFrame{Type: "bridge", Payload: body})
				return
			} else {
				warnPlatformAPIUnreachableOnce(fwdErr)
			}
		}
		if err := mockRunner.Send(payload); err != nil {
			fmt.Printf("[sandbox] mock lobby/user/ai responder error: %v\n", err)
		}
		return
	}

	resp, err := processEnvelope(runner, payload)
	if err != nil {
		fmt.Printf("[sandbox] rules runner error: %v\n", err)
		return
	}
	if resp == nil {
		return
	}
	resp = unwrapStateViewForCaller(extractPlayerID(payload), resp)
	fmt.Printf("[sandbox] -> %s\n", string(resp))
	writeFrame(devSessionFrame{Type: "bridge", Payload: resp})
}

// extractPlayerID reads envelopeJSON's own payload.playerId -- every
// multiplayer-domain request shape (PlayerIntentSchema, GetSnapshotRequest,
// EngineReadyRequest) carries one, so this works generically regardless of
// which action produced the reply being unwrapped for it.
func extractPlayerID(envelopeJSON json.RawMessage) string {
	var probe struct {
		Payload struct {
			PlayerID string `json:"playerId"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(envelopeJSON, &probe); err != nil {
		return ""
	}
	return probe.Payload.PlayerID
}

// unwrapStateViewForCaller mirrors apps/host's real per-recipient fan-out
// (see @jahandco/interactive-sdk's README: "apps/host fans a stateView
// array out to each recipientPlayerId's own socket, never broadcasting one
// object verbatim to the room"). A real deployment never puts the raw
// multiplayer.StateView[] a rules bundle returns -- stateViewsForPlayers'
// documented shape, used by every shipped multiplayer example -- on the
// wire to a single client. jc dev's dev-session tunnel has exactly one
// real client (whichever developer has the project console's Dev tab
// open) and no apps/host in the loop at all, so without this,
// BridgeClient.multiplayer.submitIntent() -- typed Promise<StateView>, a
// single object -- would resolve with the bare array instead: no crash,
// but view.publicState is always undefined and the game silently never
// updates in the console's Dev tab, even though rules.ts's own logic is
// entirely correct.
func unwrapStateViewForCaller(callerPlayerID string, respJSON []byte) []byte {
	if callerPlayerID == "" {
		return respJSON
	}

	var envelope struct {
		Domain  string          `json:"domain"`
		Action  string          `json:"action"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(respJSON, &envelope); err != nil {
		return respJSON
	}
	if envelope.Domain != "multiplayer" || envelope.Action != "stateView" {
		return respJSON
	}

	var views []json.RawMessage
	if err := json.Unmarshal(envelope.Payload, &views); err != nil {
		// Not a JSON array -- the title already replied with a single
		// StateView object, nothing to unwrap.
		return respJSON
	}

	for _, raw := range views {
		var probe struct {
			RecipientPlayerID string `json:"recipientPlayerId"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		if probe.RecipientPlayerID != callerPlayerID {
			continue
		}

		var full map[string]json.RawMessage
		if err := json.Unmarshal(respJSON, &full); err != nil {
			return respJSON
		}
		full["payload"] = raw
		rewritten, err := json.Marshal(full)
		if err != nil {
			return respJSON
		}
		return rewritten
	}

	// No entry for the caller's own playerId -- shouldn't happen for a
	// well-formed title (its own roster always includes itself), but fail
	// open rather than silently drop the reply.
	return respJSON
}

// runSession pumps frames for the lifetime of one host connection: "bridge"
// frames go through handleBridgeFrame (each in its own goroutine -- the
// rules runner and mock responder each serialize their own access
// independently, so concurrent bridge/asset traffic shouldn't block on each
// other), "asset_request" frames read straight off disk. Returns once the
// connection closes or errors.
func runSession(conn *websocket.Conn, runner *RulesRunner, mockRunner *DomainMockRunner, absPath string) {
	defer conn.Close()

	var writeMu sync.Mutex
	writeFrame := func(f devSessionFrame) {
		data, err := json.Marshal(f)
		if err != nil {
			return
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}

	mockRunner.SetOnMessage(func(payload []byte) {
		fmt.Printf("[sandbox] -> %s (mock)\n", string(payload))
		writeFrame(devSessionFrame{Type: "bridge", Payload: payload})
	})
	defer mockRunner.SetOnMessage(nil)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var f devSessionFrame
		if err := json.Unmarshal(data, &f); err != nil {
			continue
		}

		switch f.Type {
		case "bridge":
			go handleBridgeFrame(f.Payload, runner, mockRunner, writeFrame)
		case "asset_request":
			go func(requestID, path string) {
				writeFrame(serveAsset(absPath, requestID, path))
			}(f.RequestID, f.Path)
		}
	}
}

var devCmd = &cobra.Command{
	Use:   "dev [path]",
	Short: "Start a local development sandbox, viewable from the project console's Dev tab",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetPath := "."
		if len(args) > 0 {
			targetPath = args[0]
		}

		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			return err
		}

		manifest, err := readManifest(absPath)
		if err != nil {
			return err
		}
		projectID := manifest.Project.ID

		rulesJS := filepath.Join(absPath, "dist", "rules.js")
		if _, err := os.Stat(rulesJS); os.IsNotExist(err) {
			return fmt.Errorf("[jc] compiled rules not found at %s -- please run 'npm run build' in the project first", rulesJS)
		}

		fmt.Printf("[jc] starting dev session for %s\n", targetPath)

		runner, err := StartRulesRunner(rulesJS)
		if err != nil {
			return fmt.Errorf("[jc] failed to start rules evaluator: %w (make sure Node.js is installed)", err)
		}
		defer runner.Close()

		mockRunner, err := StartDomainMockRunner(absPath, filepath.Base(absPath))
		if err != nil {
			return fmt.Errorf("[jc] failed to start mock lobby/user/ai responder: %w", err)
		}
		defer mockRunner.Close()

		for {
			conn, err := dialHost(projectID)
			if err != nil {
				fmt.Printf("[jc] failed to connect dev session: %v -- retrying in 3s\n", err)
				time.Sleep(3 * time.Second)
				continue
			}

			fmt.Println("[jc] dev session connected -- open this project's console, then the Dev tab, to view it")
			runSession(conn, runner, mockRunner, absPath)
			fmt.Println("[jc] dev session disconnected -- reconnecting...")
			time.Sleep(2 * time.Second)
		}
	},
}

func init() {
	devCmd.Flags().StringVar(&playAPIURL, "play-api-url", "http://localhost:4320", "Base URL of a local play-api instance (run with PLAY_API_DEV_MODE=true) for lobby/user/ai -- set to \"\" to always use the local rules bundle instead")
	rootCmd.AddCommand(devCmd)
}
