package main

import (
	"slices"
	"testing"
)

func TestVisitKnownCities(t *testing.T) {
	raw := `SiiNunit
{
economy : economy.main {
 visited_cities: 1
 visited_cities[0]: rotterdam
 visited_cities_count: 1
 visited_cities_count[0]: 4
 last_visited_city: rotterdam
 garages: 2
 garages[0]: garage.berlin
 garages[1]: garage.ignored
 garage_ignore_list: 1
 garage_ignore_list[0]: garage.ignored
}
garage : garage.berlin {
 status: 0
}
garage : garage.ignored {
 status: 0
}
company : company.volatile.example.hamburg {
 cargo: 0
}
company : company.volatile.example.ignored {
 cargo: 0
}
}
`
	d, err := readDoc([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	economy, err := d.one("economy")
	if err != nil {
		t.Fatal(err)
	}
	additions := d.unvisitedCities(economy)
	if !slices.Equal(additions, []string{"berlin", "hamburg"}) {
		t.Fatalf("unexpected additions: %v", additions)
	}
	count, err := visitCities(&economy, additions)
	if err != nil || count != 3 {
		t.Fatalf("unexpected count %d: %v", count, err)
	}
	cities, err := readVisitArray(economy, "visited_cities")
	if err != nil || !slices.Equal(cities, []string{"rotterdam", "berlin", "hamburg"}) {
		t.Fatalf("unexpected cities: %v, %v", cities, err)
	}
	counts, err := readVisitArray(economy, "visited_cities_count")
	if err != nil || !slices.Equal(counts, []string{"4", "1", "1"}) || value(economy, "last_visited_city") != "rotterdam" {
		t.Fatalf("unexpected visit data: %v, %v", counts, err)
	}
}
