package main

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

var cityToken = regexp.MustCompile(`^[a-z0-9_]+$`)

func (d document) knownCities(economy unit) map[string]bool {
	known := make(map[string]bool)
	ignored := make(map[string]bool)
	for _, m := range ignoredGarageRefs.FindAllStringSubmatch(economy.text, -1) {
		ignored[strings.TrimPrefix(m[1], "garage.")] = true
	}
	for _, m := range garageRefs.FindAllStringSubmatch(economy.text, -1) {
		if d.byID[m[1]].kind == "garage" {
			known[strings.TrimPrefix(m[1], "garage.")] = true
		}
	}
	// Some cities have company depots but no garage of their own.
	for _, u := range d.units {
		if u.kind == "company" && strings.HasPrefix(u.id, "company.volatile.") {
			parts := strings.Split(u.id, ".")
			if len(parts) == 4 {
				known[parts[3]] = true
			}
		}
	}
	for city := range known {
		if !cityToken.MatchString(city) || ignored[city] {
			delete(known, city)
		}
	}
	return known
}

func (d document) unvisitedCities(economy unit) []string {
	known := d.knownCities(economy)
	visited, err := readEconomyArray(economy, "visited_cities")
	if err != nil {
		return nil
	}
	counts, err := readEconomyArray(economy, "visited_cities_count")
	if err != nil || len(counts) != len(visited) {
		return nil
	}
	for _, city := range visited {
		delete(known, city)
	}
	var cities []string
	for city := range known {
		cities = append(cities, city)
	}
	sort.Strings(cities)
	return cities
}

func readEconomyArray(economy unit, key string) ([]string, error) {
	field := fieldRE(key).FindString(economy.text)
	lines := strings.Split(field, "\n")
	size, err := strconv.Atoi(value(economy, key))
	if err != nil || size != len(lines)-1 {
		return nil, appError("Could not read %s.", key)
	}
	items := make([]string, 0, size)
	for i, line := range lines[1:] {
		prefix, item, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || prefix != fmt.Sprintf("%s[%d]", key, i) {
			return nil, appError("Could not read %s.", key)
		}
		items = append(items, strings.TrimSpace(item))
	}
	return items, nil
}

func appendEconomyArray(economy *unit, key string, additions []string) (int, error) {
	entries, err := readEconomyArray(*economy, key)
	if err != nil {
		return 0, err
	}
	count := len(entries)
	newCount := count + len(additions)
	err = economy.change(key, func(original, prefix string) string {
		newline := "\n"
		if strings.Contains(economy.text, "\r\n") {
			newline = "\r\n"
		}
		_, existing, hasEntries := strings.Cut(original, newline)
		var field strings.Builder
		field.WriteString(prefix + strconv.Itoa(newCount))
		if hasEntries {
			field.WriteString(newline + existing)
		}
		indent := prefix[:strings.Index(prefix, key)]
		for i, item := range additions {
			fmt.Fprintf(&field, "%s%s%s[%d]: %s", newline, indent, key, count+i, item)
		}
		return field.String()
	})
	if err != nil {
		return 0, err
	}
	updated, err := readEconomyArray(*economy, key)
	if err != nil || len(updated) != newCount || !slices.Equal(updated[count:], additions) {
		return 0, appError("Could not verify %s.", key)
	}
	return newCount, nil
}

func visitCities(economy *unit, cities []string) (int, error) {
	count, err := appendEconomyArray(economy, "visited_cities", cities)
	if err != nil {
		return 0, err
	}
	visitCounts := make([]string, len(cities))
	for i := range visitCounts {
		visitCounts[i] = "1"
	}
	countOfCounts, err := appendEconomyArray(economy, "visited_cities_count", visitCounts)
	if err != nil {
		return 0, err
	}
	if count != countOfCounts {
		return 0, appError("City visit counts do not match.")
	}
	return count, nil
}
