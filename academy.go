package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const academyFile = "academy_book_states.sii"
const academyResultVersion = "104"

type academyLesson struct {
	key   string
	rank  int
	goals string
}

type academyBook struct {
	unit
	ranks map[string]int
}

type academyWrite struct {
	Backup string `json:"backup,omitempty"`
	Added  int    `json:"added"`
}

func readAcademyBooks(d document, container unit) (map[string]academyBook, error) {
	ids, err := readUnitArray(container, "book_states")
	if err != nil {
		return nil, err
	}
	books := make(map[string]academyBook)
	for _, id := range ids {
		book := d.byID[id]
		key := value(book, "book")
		if book.kind != "academy_book_state" || key == "" || books[key].id != "" {
			return nil, appError("Could not read Driving Academy progress.")
		}
		entries, err := readUnitArray(book, "scenario_results")
		if err != nil {
			return nil, err
		}
		ranks := make(map[string]int)
		for _, id := range entries {
			result := d.byID[id]
			scenario := value(result, "scenario")
			if result.kind != "academy_scenario_result" || scenario == "" {
				return nil, appError("Could not read Driving Academy progress.")
			}
			rank, err := strconv.Atoi(value(result, "scenario_rank"))
			if err == nil && rank >= 0 && value(result, "scenario_version") == "0" &&
				value(result, "version") == academyResultVersion {
				if old, ok := ranks[scenario]; !ok || rank > old {
					ranks[scenario] = rank
				}
			}
		}
		books[key] = academyBook{book, ranks}
	}
	return books, nil
}

func cloneAcademyTruck(d document, truck string, newID func() string) (string, []string, error) {
	pending := []string{truck}
	var visited []string
	ids := make(map[string]string)
	for i := 0; i < len(pending); i++ {
		id := pending[i]
		if _, seen := ids[id]; seen {
			continue
		}
		u := d.byID[id]
		if u.kind != "vehicle" && !strings.HasPrefix(u.kind, "vehicle_") {
			return "", nil, appError("Could not copy the Driving Academy truck.")
		}
		ids[id] = newID()
		visited = append(visited, id)
		pending = append(pending, nameRE.FindAllString(u.text, -1)...)
	}
	var units []string
	for _, id := range visited {
		rewritten := nameRE.ReplaceAllStringFunc(d.byID[id].text, func(ref string) string { return ids[ref] })
		units = append(units, rewritten)
	}
	return ids[truck], units, nil
}

func academyResult(id, scenario string, lesson academyLesson, now int64) string {
	goals := strings.Fields(lesson.goals)
	score := len(goals)
	if lesson.rank == 3 {
		score += 3300
	}
	var b strings.Builder
	fmt.Fprintf(&b, "academy_scenario_result : %s {\n version: %s\n creation_time: %d\n scenario: %s\n scenario_version: 0\n scenario_score: %d\n scenario_rank: %d\n scenario_duration: 0\n",
		id, academyResultVersion, now, scenario, score, lesson.rank)
	fmt.Fprintf(&b, " goal_ids: %d\n", len(goals))
	for i, goal := range goals {
		fmt.Fprintf(&b, " goal_ids[%d]: %s\n", i, strings.TrimSuffix(goal, "+"))
	}
	fmt.Fprintf(&b, " goal_values: %d\n", len(goals))
	for i, goal := range goals {
		counter := 0
		if strings.HasSuffix(goal, "+") {
			counter = 1
		}
		fmt.Fprintf(&b, " goal_values[%d]: %d\n", i, counter)
	}
	fmt.Fprintf(&b, " goal_success: %d\n", len(goals))
	for i := range goals {
		fmt.Fprintf(&b, " goal_success[%d]: true\n", i)
	}
	b.WriteString(" trigger_state_names: 0\n trigger_state_values: 0\n}")
	return b.String()
}

func completeAcademy(d document, now int64) ([]byte, int, error) {
	container, err := d.one("academy_save_container")
	if err != nil {
		return nil, 0, err
	}
	version := value(container, "version")
	if version != "93" && version != "105" {
		return nil, 0, appError("This version of the Driving Academy save is unsupported.")
	}
	books, err := readAcademyBooks(d, container)
	if err != nil {
		return nil, 0, err
	}

	serial := 0
	newID := func() string {
		for {
			serial++
			id := fmt.Sprintf("_nameless.7ffe.7600.%04x", serial)
			if _, exists := d.byID[id]; !exists {
				return id
			}
		}
	}

	var additions, newBooks []string
	changes := make(map[string]string)
	total := 0
	for i, lessons := range academyLessons {
		bookKey := fmt.Sprintf("academy.book.book%d", i+1)
		book := books[bookKey]
		missingBook := book.id == ""
		if missingBook {
			// The three books share the same pool of trucks.
			truck := value(books["academy.book.book1"].unit, "truck")
			if d.byID[truck].kind != "vehicle" {
				return nil, 0, appError("Complete one Driving Academy lesson and close ETS2 first.")
			}
			truck, units, err := cloneAcademyTruck(d, truck, newID)
			if err != nil {
				return nil, 0, err
			}
			additions = append(additions, units...)
			book.id = newID()
			book.text = fmt.Sprintf("academy_book_state : %s {\n book: %s\n truck: %s\n scenario_results: 0\n}", book.id, bookKey, truck)
			newBooks = append(newBooks, book.id)
		}

		prefix := fmt.Sprintf("academy.scenario.book%d.", i+1)
		var results []string
		for _, lesson := range lessons {
			scenario := prefix + lesson.key
			if rank, ok := book.ranks[scenario]; ok && rank >= lesson.rank {
				continue
			}
			id := newID()
			results = append(results, id)
			additions = append(additions, academyResult(id, scenario, lesson, now))
		}
		if len(results) == 0 {
			continue
		}
		if _, err := appendUnitArray(&book.unit, "scenario_results", results); err != nil {
			return nil, 0, err
		}
		if missingBook {
			additions = append(additions, book.text)
		} else {
			changes[book.id] = book.text
		}
		total += len(results)
	}
	if total == 0 {
		return nil, 0, nil
	}
	if len(newBooks) > 0 {
		if _, err := appendUnitArray(&container, "book_states", newBooks); err != nil {
			return nil, 0, err
		}
		changes[container.id] = container.text
	}
	text := strings.TrimSpace(d.render(changes))
	end := strings.LastIndex(text, "}")
	if end < 0 || !strings.HasSuffix(strings.TrimSpace(text[:end]), "}") {
		return nil, 0, appError("Could not read Driving Academy progress.")
	}
	updated := []byte(text[:end] + "\n" + strings.Join(additions, "\n\n") + "\n}\n")
	check, err := readDoc(updated)
	if err != nil {
		return nil, 0, err
	}
	for _, id := range nameRE.FindAllString(string(updated), -1) {
		if _, ok := check.byID[id]; !ok {
			return nil, 0, appError("Could not verify Driving Academy progress.")
		}
	}
	return updated, total, nil
}

func writeAcademy(slotPath, root string, gameOpen func() (bool, error)) (academyWrite, error) {
	folder := filepath.Dir(filepath.Clean(slotPath))
	if filepath.Base(folder) != "save" {
		return academyWrite{}, appError("Choose a save inside an ETS2 profile to edit Driving Academy.")
	}
	path := filepath.Join(filepath.Dir(folder), "academy", academyFile)
	open, err := gameOpen()
	if err != nil {
		return academyWrite{}, err
	}
	if open {
		return academyWrite{}, appError("Close ETS2 before saving.")
	}
	original, err := readRegular(path)
	if errors.Is(err, os.ErrNotExist) {
		return academyWrite{}, appError("Complete one Driving Academy lesson and close ETS2 first.")
	}
	if err != nil {
		return academyWrite{}, err
	}
	d, err := readDoc(original)
	if err != nil {
		return academyWrite{}, err
	}
	updated, added, err := completeAcademy(d, time.Now().Unix())
	if err != nil || added == 0 {
		return academyWrite{}, err
	}
	staged, err := stage(path, updated)
	if err != nil {
		return academyWrite{}, err
	}
	defer os.Remove(staged)

	backup, err := makeBackup(root, "academy", backupItem{academyFile, original})
	if err != nil {
		return academyWrite{}, err
	}
	current, err := readRegular(path)
	if err != nil || !bytes.Equal(original, current) {
		return academyWrite{}, appError("The save changed. Reload it and try again.")
	}
	open, err = gameOpen()
	if err != nil {
		return academyWrite{}, err
	}
	if open {
		return academyWrite{}, appError("Close ETS2 before saving.")
	}
	if err := os.Rename(staged, path); err != nil {
		return academyWrite{}, err
	}
	return academyWrite{backup, added}, nil
}
