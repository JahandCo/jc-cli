import { Engine } from "@jahandco/game-sdk";
import { PreloaderScene } from "./scenes/Preloader";
import { GameScene } from "./scenes/Game";
import { GameOverScene } from "./scenes/GameOver";

// The only thing your title's own code ever calls to get on screen -- no
// `new Phaser.Game(config)`, no manual bridge/plugin wiring. Engine owns
// the whole boot sequence: it builds the Phaser.Game instance, boots
// straight into a working default lobby, and hands off to GameScene once
// the match starts. See PreloaderScene for where your own asset loading
// lives, and GameScene (src/scenes/Game.ts) for your actual gameplay --
// that's the only file you should need to touch to start building.
Engine.launch({
  gameId: "test-game",
  // The platform hands your title a real session token at runtime -- see
  // apps/host's own runtime page for how it's injected. "dev-session" is a
  // stand-in so this compiles/runs standalone; `jc dev`'s local tunnel
  // replaces this with a real one.
  sessionToken: "dev-session",
  parent: "game-container",
  width: 800,
  height: 600,
  title: "test-game",
  preload: PreloaderScene,
  scenes: [GameScene, GameOverScene],
});
