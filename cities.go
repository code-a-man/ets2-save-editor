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

func (d document) unvisitedCities(economy unit) []string {
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
	visited, err := readVisitArray(economy, "visited_cities")
	if err != nil {
		return nil
	}
	counts, err := readVisitArray(economy, "visited_cities_count")
	if err != nil || len(counts) != len(visited) {
		return nil
	}
	for _, city := range visited {
		delete(known, city)
	}
	var cities []string
	for city := range known {
		if cityToken.MatchString(city) && !ignored[city] {
			cities = append(cities, city)
		}
	}
	sort.Strings(cities)
	return cities
}

func readVisitArray(economy unit, key string) ([]string, error) {
	field := fieldRE(key).FindString(economy.text)
	lines := strings.Split(field, "\n")
	size, err := strconv.Atoi(value(economy, key))
	if err != nil || size != len(lines)-1 {
		return nil, fmt.Errorf("Şehir ziyaret listesi okunamadı: %s", key)
	}
	items := make([]string, 0, size)
	for i, line := range lines[1:] {
		prefix, item, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || prefix != fmt.Sprintf("%s[%d]", key, i) {
			return nil, fmt.Errorf("Şehir ziyaret listesi okunamadı: %s", key)
		}
		items = append(items, strings.TrimSpace(item))
	}
	return items, nil
}

func visitCities(economy *unit, cities []string) (int, error) {
	count, err := strconv.Atoi(value(*economy, "visited_cities"))
	if err != nil || count < 0 {
		return 0, fmt.Errorf("Şehir ziyaret sayısı okunamadı.")
	}
	newCount := count + len(cities)
	for _, key := range []string{"visited_cities", "visited_cities_count"} {
		err := economy.change(key, func(original, prefix string) string {
			newline := "\n"
			if strings.Contains(economy.text, "\r\n") {
				newline = "\r\n"
			}
			_, entries, hasEntries := strings.Cut(original, newline)
			var field strings.Builder
			field.WriteString(prefix + strconv.Itoa(newCount))
			if hasEntries {
				field.WriteString(newline + entries)
			}
			indent := prefix[:strings.Index(prefix, key)]
			for i, city := range cities {
				item := city
				if key == "visited_cities_count" {
					item = "1"
				}
				fmt.Fprintf(&field, "%s%s%s[%d]: %s", newline, indent, key, count+i, item)
			}
			return field.String()
		})
		if err != nil {
			return 0, err
		}
	}
	visited, err := readVisitArray(*economy, "visited_cities")
	if err != nil || len(visited) != newCount || !slices.Equal(visited[count:], cities) {
		return 0, fmt.Errorf("Şehir ziyaretleri doğrulanamadı.")
	}
	return newCount, nil
}
