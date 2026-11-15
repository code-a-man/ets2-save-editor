# ETS2 Save Editor

A macOS save editor for Euro Truck Simulator 2. Edit your balance, level, XP and skills. Repair or refuel your truck, open garages, unlock quick travel cities and discover truck dealerships.

## Install and use

1. [Download the macOS ZIP](https://github.com/code-a-man/ets2-save-editor/releases/latest/download/ETS2-Save-Editor-macOS-universal.zip). Unzip it and move **ETS2 Save Editor.app** to Applications. Built for macOS 13 and later on Apple Silicon and Intel.
2. Close ETS2. The editor finds local and Steam Cloud saves. Use **Choose…** for a custom folder containing `game.sii` and `info.sii`.
3. Make your changes, click **Save**, then use **Launch Game** and load the edited save in ETS2.

If macOS blocks the first launch, try opening the app once, then use **System Settings → Privacy & Security → Open Anyway**. For later releases, click **Check for Updates…** in the main window.

**Complete Driving Academy** fills in the three Academy modules for the selected profile. Complete one lesson in ETS2 first to create the progress file.

Backups are stored in `~/Library/Application Support/ETS2 Save Editor/Backups/`. To restore one, close ETS2 and return the ZIP’s SII files to the original save or Academy folder.

**Steam Settings → Vulkan + SDR** changes the launch options while Steam is closed. At 100% scaling Vulkan can show a black screen. Use a different scaling value in ETS2.

## Build

Use macOS with Xcode Command Line Tools and Go 1.24 or later.

```sh
go test ./...
./scripts/build.sh
open "dist/ETS2 Save Editor.app"
```

The repository owner publishes by pushing a `v*` tag. GitHub Actions builds the universal ZIP, then the release job signs and publishes it. The signing key is stored in the protected `release` environment.

[MIT license](LICENSE) · Save decoding by [ets2-decrypt](https://github.com/avoinescu/ets2-decrypt) · Updates by [Sparkle](https://sparkle-project.org/)
