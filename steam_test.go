package main

import (
	"strings"
	"testing"
)

func TestSteamLaunchOptions(t *testing.T) {
	// Other apps and escaped launch arguments must survive a round trip.
	const raw = `"UserLocalConfigStore" { "Software" { "Valve" { "Steam" { "apps" {
		"123" { "LaunchOptions" "-rdevice vk -color_mode sdr" }
		"227300" {
			"LaunchOptions" "-nointro -homedir \"/Volumes/Drive A/ETS2\""
		}
	} } } } }`
	on, enabled, err := patchSteamConfig(raw, "on")
	if err != nil || !enabled || !strings.Contains(on, `-homedir \"/Volumes/Drive A/ETS2\" `+fpsOptions) {
		t.Fatalf("failed to enable: %v %s", err, on)
	}
	off, enabled, err := patchSteamConfig(on, "off")
	if err != nil || enabled || off != raw {
		t.Fatalf("round trip changed unrelated settings: %v %s", err, off)
	}
	missing := strings.Replace(raw, `"LaunchOptions" "-nointro -homedir \"/Volumes/Drive A/ETS2\""`, "", 1)
	added, enabled, err := patchSteamConfig(missing, "on")
	if err != nil || !enabled || !strings.Contains(added, `"LaunchOptions"		"`+fpsOptions+`"`) {
		t.Fatalf("could not add missing option: %v %s", err, added)
	}
	conflict := strings.Replace(raw, "-nointro", "-rdevice gl", 1)
	if _, _, err := patchSteamConfig(conflict, "on"); err == nil {
		t.Fatal("different renderer would be overwritten")
	}
}
