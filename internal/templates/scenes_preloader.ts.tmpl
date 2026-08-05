import Phaser from "phaser";
import { LOBBY_SCENE_KEY } from "@jahandco/game-sdk";

// Loads this title's real assets and shows progress while it does --
// canvas-drawn (Phaser Graphics), same convention every official Phaser
// template uses. Engine runs this after its own internal boot scene and
// before the default lobby -- it's the one piece of the pre-lobby chain
// that's genuinely title-specific (what to load), so it's still yours to
// author, unlike Phaser.Game construction or the lobby itself.
export class PreloaderScene extends Phaser.Scene {
  constructor() {
    super("Preloader");
  }

  preload(): void {
    // Everything under public/assets/ at your project root ships inside
    // the deploy bundle -- webpack.client.js.tmpl's CopyPlugin copies it
    // into out/assets/ as part of `npm run build`/`npm run dev`. Reference
    // paths relative to that directory.
    this.load.setPath("assets");

    // this.load.image("logo", "logo.png");
    // this.load.spritesheet("player", "player.png", { frameWidth: 32, frameHeight: 32 });
    // this.load.audio("theme", "theme.mp3");

    const { width, height } = this.cameras.main;
    const barWidth = 320;
    const barHeight = 24;
    const barX = width / 2 - barWidth / 2;
    const barY = height / 2 - barHeight / 2;

    this.add.rectangle(width / 2, height / 2, barWidth + 8, barHeight + 8, 0x1a2033);
    const bar = this.add.graphics();

    this.load.on("progress", (progress: number) => {
      bar.clear();
      bar.fillStyle(0x5b7cff, 1);
      bar.fillRect(barX, barY, barWidth * progress, barHeight);
    });
  }

  create(): void {
    // Engine's default lobby (or your own LobbyClient subclass, if you
    // configured Engine.launch's `lobby.class`) runs next -- join/ready/
    // chat/host controls, then a hand-off into GameScene once the match
    // starts. See @jahandco/game-sdk's LobbyClient if you want to
    // customize it instead of using the stock lobby.
    this.scene.start(LOBBY_SCENE_KEY);
  }
}
