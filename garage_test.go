package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestGaragePurchasePreservesExistingSlots(t *testing.T) {
	d, err := readDoc([]byte(`SiiNunit
{
economy : economy.main {
 garages: 3
 garages[0]: garage.berlin
 garages[1]: garage.rotterdam
 garages[2]: garage.missing_dlc
 garage_ignore_list: 1
 garage_ignore_list[0]: garage.missing_dlc
}
garage : garage.berlin {
 vehicles: 0
 drivers: 0
 trailers: 0
 status: 0
 productivity: 0
}
garage : garage.rotterdam {
 vehicles: 1
 vehicles[0]: truck.001
 drivers: 1
 drivers[0]: driver.001
 trailers: 0
 status: 6
 productivity: 0.5
}
garage : garage.missing_dlc {
 vehicles: 0
 drivers: 0
 trailers: 0
 status: 0
}
}
`))
	if err != nil {
		t.Fatal(err)
	}
	economy, err := d.one("economy")
	if err != nil {
		t.Fatal(err)
	}
	choices := d.garages(economy)
	if len(choices) != 3 || choices[0].ID != "garage.berlin" || !choices[0].CanBuy || choices[1].CanBuy || choices[2].CanBuy {
		t.Fatalf("unexpected garage choices: %+v", choices)
	}
	for _, id := range []string{"garage.rotterdam", "garage.missing_dlc", "garage.unknown"} {
		if err := d.buyGarages(economy, []string{id}, map[string]string{}); err == nil {
			t.Fatalf("unexpectedly allowed purchase of %s", id)
		}
	}
	changes := make(map[string]string)
	if err := d.buyGarages(economy, []string{"garage.berlin"}, changes); err != nil {
		t.Fatal(err)
	}
	next, err := readDoc([]byte(d.render(changes)))
	if err != nil {
		t.Fatal(err)
	}
	g := next.byID["garage.berlin"]
	if value(g, "status") != "3" || value(g, "vehicles") != "5" || value(g, "drivers") != "5" || value(g, "trailers") != "0" {
		t.Fatal(g.text)
	}
	for _, field := range []string{"vehicles", "drivers"} {
		for i := 0; i < 5; i++ {
			if !strings.Contains(g.text, fmt.Sprintf(" %s[%d]: null\n", field, i)) {
				t.Fatalf("missing %s slot %d", field, i)
			}
		}
	}
	for id, u := range d.byID {
		if id != g.id && next.byID[id].text != u.text {
			t.Fatalf("purchase modified %s", id)
		}
	}
}
