package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const maxXP = 2147483647

// ETS2 1.62 def/economy_data_xp.sii stores increments. From level 30 on,
// each additional level uses the last increment.
var truckXP = [...]int{200, 500, 700, 900, 1000, 1100, 1300, 1600, 1700, 2100,
	2300, 2600, 2700, 2900, 3000, 3100, 3400, 3700, 4000, 4300,
	4600, 4700, 4900, 5200, 5700, 5900, 6000, 6200, 6600, 6800}

func xpForLevel(level int) int {
	xp := 0
	for i, increment := range truckXP {
		if i == level {
			return xp
		}
		xp += increment
	}
	return xp + (level-len(truckXP))*truckXP[len(truckXP)-1]
}

func levelForXP(xp int) int {
	for i, increment := range truckXP {
		if xp < increment {
			return i
		}
		xp -= increment
	}
	return len(truckXP) + xp/truckXP[len(truckXP)-1]
}

func targetXP(current int, target string) (int, error) {
	if target == "keep" {
		return current, nil
	}
	mode, input, _ := strings.Cut(target, ":")
	n, err := strconv.Atoi(input)
	if err != nil || n < 0 {
		return 0, errors.New("Seviye veya XP için pozitif bir tam sayı ya da 0 gir.")
	}
	switch mode {
	case "level":
		if n <= levelForXP(maxXP) {
			if n == levelForXP(current) {
				return current, nil
			}
			return xpForLevel(n), nil
		}
	case "xp":
		if n <= maxXP {
			return n, nil
		}
	}
	return 0, errors.New("Seviye veya XP desteklenen aralığın dışında.")
}

type activeRig struct {
	truck    unit
	trailers []unit
}

func (d document) playerRig() activeRig {
	player, err := d.one("player")
	if err != nil {
		return activeRig{}
	}
	job := d.byID[value(player, "current_job")]
	if onlineID := value(job, "online_job_id"); onlineID != "" && onlineID != "nil" && onlineID != "0" {
		return activeRig{}
	}
	var truck, trailer unit
	for _, mode := range []string{"assigned", "my"} {
		// New saves store a vehicle and trailer in a player_vehicles unit.
		vehicles := d.byID[value(player, mode+"_vehicles")]
		if value(player, mode+"_vehicles_mode") == "truck" && vehicles.kind == "player_vehicles" {
			truck, trailer = d.byID[value(vehicles, "vehicle")], d.byID[value(vehicles, "trailer")]
		}
		// Older saves reference the truck and trailer directly.
		if truck.kind != "vehicle" {
			truck, trailer = d.byID[value(player, mode+"_truck")], d.byID[value(player, mode+"_trailer")]
		}
		if truck.kind == "vehicle" {
			break
		}
	}
	if truck.kind != "vehicle" {
		return activeRig{}
	}
	rig := activeRig{truck: truck}
	seen := make(map[string]bool)
	for trailer.kind == "trailer" && !seen[trailer.id] {
		rig.trailers = append(rig.trailers, trailer)
		seen[trailer.id] = true
		trailer = d.byID[value(trailer, "slave_trailer")]
	}
	return rig
}

var wearField = regexp.MustCompile(`(?m)^([ \t]+(?:engine_wear|transmission_wear|cabin_wear|chassis_wear|trailer_body_wear|wheels_wear(?:_unfixable)?\[\d+\]|(?:engine|transmission|cabin|chassis|trailer_body)_wear_unfixable):[ \t]*)[^\r\n]+`)

func (r activeRig) edit(changes map[string]string, repair, refuel bool) error {
	if r.truck.id == "" || (refuel && value(r.truck, "fuel_relative") == "") {
		return fmt.Errorf("Bu kayıtta düzenlenebilir bir tır bulunamadı.")
	}
	truck := r.truck
	if refuel {
		if err := truck.set("fuel_relative", "1"); err != nil {
			return err
		}
	}
	if repair {
		truck.text = wearField.ReplaceAllString(truck.text, "${1}0")
		for _, trailer := range r.trailers {
			changes[trailer.id] = wearField.ReplaceAllString(trailer.text, "${1}0")
		}
	}
	changes[truck.id] = truck.text
	return nil
}
