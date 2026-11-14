package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverSavesAcrossLocations(t *testing.T) {
	home := filepath.Join(t.TempDir(), "mac[user]")
	support := filepath.Join(home, "Library", "Application Support")
	cloud := filepath.Join(support, "Steam", "userdata", "123", "227300", "remote", "profiles", "4d6574", "save")
	local := filepath.Join(home, "Documents", "Euro Truck Simulator 2", "steam_profiles", "54657374", "save")
	earlier := time.Unix(100, 0)
	later := time.Unix(200, 0)
	add := func(root, name string, mtime time.Time, includeInfo bool) string {
		t.Helper()
		slot := filepath.Join(root, name)
		if err := os.MkdirAll(slot, 0700); err != nil {
			t.Fatal(err)
		}
		game := filepath.Join(slot, "game.sii")
		if err := os.WriteFile(game, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(game, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		if includeInfo {
			if err := os.WriteFile(filepath.Join(slot, "info.sii"), nil, 0600); err != nil {
				t.Fatal(err)
			}
		}
		return slot
	}
	first := add(cloud, "autosave", later, true)
	second := add(local, "3", earlier, true)
	add(cloud, "multiplayer_backup_1", later, true)
	add(local, "partial", later, false)
	if err := os.Symlink(first, filepath.Join(local, "duplicate")); err != nil {
		t.Fatal(err)
	}
	saves := discoverSaves(home)
	if len(saves) != 2 || saves[0].Path != first || saves[1].Path != second {
		t.Fatalf("unexpected saves: %+v", saves)
	}
}
