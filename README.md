# ETS2 Save Editor

A macOS save editor for Euro Truck Simulator 2. Edit your balance, level, XP and skills. Repair or refuel your truck, open garages, unlock quick travel cities and discover truck dealerships.

## Install and use

1. [Download the macOS ZIP](https://github.com/code-a-man/ets2-save-editor/releases/latest/download/ETS2-Save-Editor-macOS-universal.zip). Unzip it and move **ETS2 Save Editor.app** to Applications. Built for macOS 13 and later on Apple Silicon and Intel.
2. Close ETS2. The editor finds local and Steam Cloud saves. Use **Choose…** for a custom folder containing `game.sii` and `info.sii`.
3. Make your changes, click **Save**, and reload that save in ETS2.

If macOS blocks the first launch, try opening the app once, then use **System Settings → Privacy & Security → Open Anyway**. For later releases, use **ETS2 Save Editor → Check for Updates…**.

Backups are stored in `~/Library/Application Support/ETS2 Save Editor/Backups/`. To restore one, close ETS2 and copy the ZIP’s two SII files back to the original save folder.

**Steam Settings → Vulkan + SDR** changes the launch options while Steam is closed. At 100% scaling Vulkan can show a black screen. Use a different scaling value in ETS2.

## Build

Use macOS with Xcode Command Line Tools and Go 1.24 or later.

```sh
go test ./...
./scripts/build.sh
open "dist/ETS2 Save Editor.app"
```

Pushing a version tag triggers the GitHub Actions release workflow. It builds the universal ZIP and signs the Sparkle update with the repository’s `SPARKLE_SIGNING_KEY` secret.

[MIT license](LICENSE) · Save decoding by [ets2-decrypt](https://github.com/avoinescu/ets2-decrypt) · Updates by [Sparkle](https://sparkle-project.org/)
