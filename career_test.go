package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCareerAndActiveRig(t *testing.T) {
	dir := t.TempDir()
	game := `SiiNunit
{
economy : economy.main {
 experience_points: 460
 adr: 0
 long_dist: 0
 heavy: 1
 fragile: 0
 urgent: 0
 mechanical: 0
}
bank : bank.main {
 money_account: 7188
}
player : player.main {
 assigned_vehicles_mode: truck
 assigned_vehicles: vehicles.main
 current_job: null
}
player_vehicles : vehicles.main {
 vehicle: truck.active
 trailer: trailer.active
}
vehicle : truck.active {
 fuel_relative: &3ec00000
 engine_wear: 1
 cabin_wear_unfixable: &3d000000
 wheels_wear: 2
 wheels_wear[0]: 1
 wheels_wear[1]: 1
}
trailer : trailer.active {
 trailer_body_wear: 1
 cargo_damage: 1
 slave_trailer: trailer.extra
}
trailer : trailer.extra {
 chassis_wear: 1
 slave_trailer: null
}
vehicle : truck.shop {
 fuel_relative: 0
 engine_wear: 1
}
}
`
	info := "SiiNunit\n{\nsave_container : info.main {\n info_players_experience: 460\n info_money_account: 7188\n}\n}\n"
	for filename, data := range map[string]string{"game.sii": game, "info.sii": info} {
		if err := os.WriteFile(filepath.Join(dir, filename), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s, err := readSlot(dir)
	if err != nil {
		t.Fatal(err)
	}
	closed := func() (bool, error) { return false, nil }
	args := []string{s.token, "7188", "0", "0", "1", "0", "0", "0", "keep", "level:30", "true", "true"}
	v, err := write(s, args, filepath.Join(dir, "backups"), closed)
	if err != nil {
		t.Fatal(err)
	}
	if v.XP != 99700 || v.Level != 30 || !v.CanRepair || !v.CanRefuel {
		t.Fatalf("unexpected saved state: %+v", v)
	}
	updated, err := readSlot(dir)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := updated.info.one("save_container")
	if err != nil || value(metadata, "info_players_experience") != "99700" {
		t.Fatal("XP was not saved to both files")
	}
	for id, expected := range map[string][]string{
		"truck.active":   {"fuel_relative: 1", "engine_wear: 0", "cabin_wear_unfixable: 0", "wheels_wear: 2", "wheels_wear[1]: 0"},
		"trailer.active": {"trailer_body_wear: 0", "cargo_damage: 1"},
		"trailer.extra":  {"chassis_wear: 0"},
	} {
		for _, line := range expected {
			if !bytes.Contains([]byte(updated.game.byID[id].text), []byte(line+"\n")) {
				t.Fatalf("%s: expected %s", id, line)
			}
		}
	}
	if updated.game.byID["truck.shop"].text != s.game.byID["truck.shop"].text {
		t.Fatal("dealer truck was modified")
	}
	if same, err := targetXP(460, "level:1"); same != 460 || err != nil {
		t.Fatal("saving the current level dropped progress toward the next one")
	}
	if xp, err := targetXP(0, "level:100"); xp != 575700 || err != nil {
		t.Fatal("high-level XP threshold is wrong")
	}
}
