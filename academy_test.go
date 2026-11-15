package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAcademy(t *testing.T) {
	const initial = `SiiNunit
{
academy_save_container : _nameless.1 {
 version: 105
 book_states: 1
 book_states[0]: _nameless.2
 log: _nameless.3
}

academy_book_state : _nameless.2 {
 book: academy.book.book1
 truck: _nameless.4
 scenario_results: 1
 scenario_results[0]: _nameless.6
}

academy_log : _nameless.3 {
 entries: 0
}

vehicle : _nameless.4 {
 accessories: 1
 accessories[0]: _nameless.5
}

vehicle_accessory : _nameless.5 {
 data_path: "/def/vehicle/truck/academy.sii"
}

academy_scenario_result : _nameless.6 {
 version: 104
 scenario: academy.scenario.book1.intro
 scenario_version: 0
 scenario_rank: 0
}
}
`
	root := t.TempDir()
	dir := filepath.Join(root, "profile", "academy")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, academyFile)
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	slot := filepath.Join(root, "profile", "save", "autosave")
	backups := filepath.Join(root, "backups")
	if _, err := writeAcademy(slot, backups, func() (bool, error) { return true, nil }); err == nil {
		t.Fatal("accepted an edit while the game was open")
	}
	closed := func() (bool, error) { return false, nil }
	result, err := writeAcademy(slot, backups, closed)
	if err != nil || result.Added != 77 || result.Backup == "" {
		t.Fatalf("writeAcademy = %+v, %v", result, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	d, err := readDoc(raw)
	if err != nil {
		t.Fatal(err)
	}
	container, err := d.one("academy_save_container")
	if err != nil {
		t.Fatal(err)
	}
	books, err := readAcademyBooks(d, container)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 3 || len(books["academy.book.book1"].ranks) != 32 ||
		len(books["academy.book.book2"].ranks) != 27 || len(books["academy.book.book3"].ranks) != 19 {
		t.Fatalf("incorrect Academy completion counts")
	}
	if rank := books["academy.book.book2"].ranks["academy.scenario.book2.zero"]; rank != 3 {
		t.Fatalf("star-rated lesson was saved with rank %d", rank)
	}
	if d.byID["_nameless.6"].kind != "academy_scenario_result" || value(container, "log") != "_nameless.3" {
		t.Fatal("existing log or result was lost")
	}
	second, err := writeAcademy(slot, backups, closed)
	if err != nil || second.Added != 0 || second.Backup != "" {
		t.Fatalf("repeat writeAcademy = %+v, %v", second, err)
	}
	archive, err := zip.OpenReader(result.Backup)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 1 || archive.File[0].Name != "academy/"+academyFile {
		t.Fatal("unexpected Academy backup contents")
	}
	entry, err := archive.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer entry.Close()
	saved, err := io.ReadAll(entry)
	if err != nil || !bytes.Equal(saved, []byte(initial)) {
		t.Fatal("backup does not match the original")
	}
}
