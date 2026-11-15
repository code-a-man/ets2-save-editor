package main

import (
	"slices"
	"testing"
)

func TestDiscoverAvailableDealers(t *testing.T) {
	d, err := readDoc([]byte(`SiiNunit
{
economy : economy.main {
 unlocked_dealers: 1
 unlocked_dealers[0]: osnabruck
 garages: 2
 garages[0]: garage.berlin
 garages[1]: garage.reykjavik
 garage_ignore_list: 1
 garage_ignore_list[0]: garage.reykjavik
}
garage : garage.berlin {
 status: 0
}
garage : garage.reykjavik {
 status: 0
}
company : company.volatile.terminal.osnabruck {
 cargo: 0
}
company : company.volatile.terminal.hamburg {
 cargo: 0
}
company : company.volatile.terminal.travemunde {
 cargo: 0
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
	pending := d.undiscoveredDealers(economy)
	if !slices.Equal(pending, []string{"berlin", "hamburg"}) {
		t.Fatalf("unexpected dealer candidates: %v", pending)
	}
	count, err := appendUnitArray(&economy, "unlocked_dealers", pending)
	if err != nil || count != 3 {
		t.Fatalf("unexpected dealer count %d: %v", count, err)
	}
	dealers, err := readUnitArray(economy, "unlocked_dealers")
	if err != nil || !slices.Equal(dealers, []string{"osnabruck", "berlin", "hamburg"}) {
		t.Fatalf("unexpected dealer list: %v, %v", dealers, err)
	}
}
