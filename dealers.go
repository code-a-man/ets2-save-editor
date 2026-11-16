package main

import (
	"slices"
	"sort"
	"strings"
)

// Dealer city tokens from the vehicle_brands fields in ETS2 1.62 city definitions.
// Match them against the selected save so inactive DLC cities are never added.
var dealerCities = map[string]string{
	"daf":      "amsterdam bergen bordeaux bremen burgos cardiff catania glasgow iasi kokkola leipzig maribor patras petersburg salzburg sevilla skopje tartu thessaloniki trondheim zurich",
	"iveco":    "almeria beograd brussel bucuresti constanta frankfurt galati gijon grimsby hamburg helsinki klaipeda kobenhavn kuopio lemans roma torino veli_tarnovo wien zaragoza",
	"man":      "ajaccio berlin bern birmingham calais cluj_napoca cordoba dortmund edinburgh ioannina krakow linkoping madrid marseille munchen porto riga rijeka split taranto tirana tuzla",
	"mercedes": "alesund badajoz bologna bratislava geneve istanbul kaliningrad kalmar lulea nantes newcastle plymouth podgorica rotterdam rovaniemi sassari stuttgart szczecin valladolid warszawa",
	"renault":  "barcelona bourges budapest duisburg edirne felixstowe kaunas kragujevac leon ljubljana lyon oslo palermo paris prague rostock sofia szeged toulouse",
	"scania":   "aberdeen banja_luka bayonne borlange brest cagliari dresden firenze gdansk hannover lahti lamia lille lisboa manchester milano oulu pitesti stockholm strasbourg tallinn zagreb",
	"volvo":    "albacete brasov goteborg graz joensuu kalamata lillehammer limoges london luxembourg malaga napoli narvik nurnberg osnabruck plovdiv pristina reykjavik turku verona vilnius wroclaw",
}

func (d document) undiscoveredDealers(economy unit) []string {
	unlocked, err := readUnitArray(economy, "unlocked_dealers")
	if err != nil {
		return nil
	}
	cities := d.knownCities(economy)
	var dealers []string
	for _, tokens := range dealerCities {
		for _, city := range strings.Fields(tokens) {
			if !cities[city] {
				continue
			}
			if dealers == nil {
				dealers = []string{}
			}
			if !slices.Contains(unlocked, city) {
				dealers = append(dealers, city)
			}
			delete(cities, city)
		}
	}
	sort.Strings(dealers)
	return dealers
}
