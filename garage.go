package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type garageView struct {
	ID     string `json:"id"`
	City   string `json:"city"`
	Status string `json:"status"`
	CanBuy bool   `json:"canBuy"`
}

var garageRefs = regexp.MustCompile(`(?m)^[ \t]+garages\[\d+\]:[ \t]*(garage\.[^\s]+)`)
var ignoredGarageRefs = regexp.MustCompile(`(?m)^[ \t]+garage_ignore_list\[\d+\]:[ \t]*(garage\.[^\s]+)`)
var occupiedGarageSlots = regexp.MustCompile(`(?m)^[ \t]+(?:vehicles|drivers|trailers)\[\d+\]:`)

func (d document) garages(economy unit) []garageView {
	garages := make([]garageView, 0)
	ignored, seen := make(map[string]bool), make(map[string]bool)
	for _, match := range ignoredGarageRefs.FindAllStringSubmatch(economy.text, -1) {
		ignored[match[1]] = true
	}
	for _, match := range garageRefs.FindAllStringSubmatch(economy.text, -1) {
		id := match[1]
		g := d.byID[id]
		if seen[id] || g.kind != "garage" {
			continue
		}
		seen[id] = true
		status := value(g, "status")
		garages = append(garages, garageView{
			ID: id, City: strings.ReplaceAll(strings.TrimPrefix(id, "garage."), "_", " "), Status: status,
			CanBuy: status == "0" && !ignored[id] && value(g, "vehicles") == "0" &&
				value(g, "drivers") == "0" && value(g, "trailers") == "0" &&
				!occupiedGarageSlots.MatchString(g.text),
		})
	}
	sort.Slice(garages, func(i, j int) bool { return garages[i].City < garages[j].City })
	return garages
}

func (d document) buyGarages(economy unit, ids []string, changes map[string]string) error {
	buyable := make(map[string]bool)
	for _, g := range d.garages(economy) {
		buyable[g.ID] = g.CanBuy
	}
	for _, id := range ids {
		if !buyable[id] {
			return appError("Could not purchase garage %s.", id)
		}
		delete(buyable, id)
		g := d.byID[id]
		// Status 3 is a large garage. Both arrays must have five slots.
		if err := g.set("status", "3"); err != nil {
			return err
		}
		newline := "\n"
		if strings.Contains(g.text, "\r\n") {
			newline = "\r\n"
		}
		for _, key := range []string{"vehicles", "drivers"} {
			err := g.change(key, func(_, prefix string) string {
				indent := prefix[:strings.Index(prefix, key)]
				var slots strings.Builder
				slots.WriteString(prefix + "5")
				for i := 0; i < 5; i++ {
					fmt.Fprintf(&slots, "%s%s%s[%d]: null", newline, indent, key, i)
				}
				return slots.String()
			})
			if err != nil {
				return err
			}
		}
		changes[g.id] = g.text
	}
	return nil
}
