import Phaser from "phaser";

// Reached when GameScene's onRoomEvent sees a roomClosed event (the room's
// session ended -- see @jahandco/interactive-protocol's
// multiplayer.RoomEvent). There's no "MainMenu" scene to restart into --
// Engine's default lobby only ever runs once, at boot. Starting a new
// match means reloading the page and forming a new lobby, same as leaving
// and returning to any other title.
export class GameOverScene extends Phaser.Scene {
  constructor() {
    super("GameOver");
  }

  create(): void {
    const { width, height } = this.cameras.main;

    this.add.text(width / 2, height / 2, "Game Over", {
      fontFamily: "monospace",
      fontSize: "28px",
      color: "#e6e9f0",
    }).setOrigin(0.5);
  }
}
