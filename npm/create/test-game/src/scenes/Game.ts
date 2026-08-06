import Phaser from "phaser";
import { GameClient } from "@jahandco/game-sdk";
import type { multiplayer } from "@jahandco/interactive-protocol";

// Your title's actual gameplay. GameClient (see @jahandco/game-sdk) already
// wires the engineReady signal, a late-joiner snapshot request, and the
// onStateView/onRoomEvent subscriptions -- implement onStart()/playerId()
// for your own setup, and override onStateView/onRoomEvent/onSnapshot
// below to actually render. src/rules.ts is the sole authority on what's
// actually true; this scene only ever renders and predicts, submitting
// player intents via this.submitIntent() (or this.jc.multiplayer directly)
// for rules.ts's handleIntent to resolve.
export class GameScene extends GameClient {
  private log?: Phaser.GameObjects.Text;

  constructor() {
    super("Game");
  }

  protected onStart(): void {
    this.log = this.add.text(16, 16, "waiting…", { fontFamily: "monospace", fontSize: "14px" });

    this.input.on("pointerdown", () => {
      this.submitIntent("click").catch((err: unknown) => {
        this.log?.setText(`error: ${err instanceof Error ? err.message : String(err)}`);
      });
    });
  }

  // Called every frame -- override instead of Phaser's own update(). Read
  // input and call this.submitIntent(...) here for anything that should
  // affect authoritative state, alongside whatever rendering/interpolation
  // this frame needs.
  protected onTick(_time: number, _delta: number): void {}

  protected onStateView(view: multiplayer.StateView): void {
    this.log?.setText(JSON.stringify(view, null, 2));
  }

  protected onSnapshot(view: multiplayer.StateView): void {
    this.onStateView(view);
  }

  protected onRoomEvent(event: multiplayer.RoomEvent): void {
    if (event.type === "roomClosed") {
      this.scene.start("GameOver");
    }
  }

  // The real userId, resolved from the lobby's own member list at the
  // Start Game handoff -- Engine/LobbyClient populate the "playerId"
  // registry key once, before this scene's create() ever runs.
  protected playerId(): string {
    return (this.registry.get("playerId") as string | undefined) ?? "solo-player";
  }
}
