import type { BridgeEnvelope, WorkerContext } from "@jahandco/interactive-protocol";
import { reply } from "@jahandco/game-sdk/rules-kit";

// Your title's server-side game logic -- the "Rules" side of the
// platform's Intent/Rules split. This runs inside a session-scoped V8
// isolate (see SESSION_ISOLATES.md in the platform repo, if you're
// curious) -- one call per player intent, mediated the same way for every
// title regardless of what kind of game this is. There's no fixed
// structure this has to follow beyond the shape of this function itself.
//
// envelope.payload is whatever your client sent via this.submitIntent()
// (GameScene, extending @jahandco/game-sdk's GameClient) -- shape it
// however your game needs.
//
// context gives you:
//   context.auth.userId    -- the server-resolved caller identity
//   context.roster         -- the match's player ids, frozen once at lobby
//                              start (absent if no match/roster exists yet)
//   context.services.db    -- mediated, policy-checked persistence
//                              (get/set against a shared store -- see
//                              BAAS_PIVOT.md Phase D if you need it)
export function handleIntent(envelope: BridgeEnvelope, context: WorkerContext): BridgeEnvelope | undefined {
  // Keep this branch even if you delete everything else below --
  // apps/host's client-side loading screen only clears on a
  // multiplayer/roomEvent broadcast carrying {type: "engineReady"}, and
  // nothing in the platform sends that automatically. Without it, a
  // client whose GameClient scene has already signaled engineReady gets
  // stuck on the loading screen forever, even though everything else
  // works.
  if (envelope.domain === "multiplayer" && envelope.action === "engineReady") {
    const req = envelope.payload as { playerId: string };
    return reply(envelope, "multiplayer", "roomEvent", { type: "engineReady", playerId: req.playerId });
  }

  return undefined;
}
