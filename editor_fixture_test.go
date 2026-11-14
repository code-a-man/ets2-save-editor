package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineQuickJobAndSkills(t *testing.T) {
	fixture := os.Getenv("ETS2_MINI_FIXTURE")
	if fixture == "" {
		t.Skip("Set ETS2_MINI_FIXTURE to test real saves")
	}
	work := t.TempDir()
	for _, folder := range []string{"autosave", "autosave_job_2"} {
		dir := filepath.Join(work, folder)
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"game.sii", "info.sii"} {
			raw, err := os.ReadFile(filepath.Join(fixture, folder, name))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	original, err := readSlot(filepath.Join(work, "autosave"))
	if err != nil {
		t.Fatal(err)
	}
	initial, err := inspect(original)
	if err != nil || !initial.CanCancel || initial.Money != "7188" {
		t.Fatalf("inspect original: %+v, %v", initial, err)
	}

	args := []string{initial.Token, "20000", "1", "1", "2", "1", "1", "1", "cancel"}
	root := filepath.Join(work, "backups")
	open := func() (bool, error) { return true, nil }
	closed := func() (bool, error) { return false, nil }
	if _, err := write(original, args, root, open); err == nil {
		t.Fatal("open game did not prevent write")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("open game created a backup")
	}
	after, err := write(original, args, root, closed)
	if err != nil {
		t.Fatal(err)
	}
	if after.Money != "20000" || after.Skills[2] != 2 || after.Skills[0] != 1 || after.Job != "" {
		t.Fatalf("unexpected result: %+v", after)
	}

	saved, err := readSlot(original.path)
	if err != nil {
		t.Fatal(err)
	}
	player, _ := saved.game.one("player")
	economy, _ := saved.game.one("economy")
	oldEconomy, _ := original.game.one("economy")
	metadata, _ := saved.info.one("save_container")
	if value(player, "current_job") != "null" || value(economy, "experience_points") != value(oldEconomy, "experience_points") ||
		value(economy, "game_time") != value(oldEconomy, "game_time") || value(metadata, "info_money_account") != "20000" {
		t.Fatal("saved state did not match the requested changes")
	}
	for _, u := range original.game.units {
		if u.kind == "driver_ai" && saved.game.byID[u.id].text != u.text {
			t.Fatal("AI driver's skills changed")
		}
	}
	archive, err := zip.OpenReader(after.Backup)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for i, want := range [][]byte{original.rawGame, original.rawInfo} {
		r, err := archive.File[i].Open()
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("backup differs from original")
		}
	}
	if _, err := write(saved, args, root, closed); err == nil {
		t.Fatal("stale edit was accepted")
	}
	reopened, err := readSlot(original.path)
	if err != nil || reopened.token != after.Token {
		t.Fatal("stale edit modified save")
	}
}
