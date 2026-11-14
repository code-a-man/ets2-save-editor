package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const fpsOptions = "-color_mode sdr -rdevice vk"

type steamSettings struct {
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
	Token   string `json:"token"`
}

type vdfEntry struct {
	key, value string
	start, end int
	children   []vdfEntry
}

func (e vdfEntry) child(key string) vdfEntry {
	for _, item := range e.children {
		if strings.EqualFold(item.key, key) {
			return item
		}
	}
	return vdfEntry{}
}

var vdfToken = regexp.MustCompile(`"(?:\\.|[^"\\])*"|//[^\r\n]*|[^\s{}"]+|[{}]`)
var optionToken = regexp.MustCompile(`(?:[^\s"'\\]|"(?:\\.|[^"\\])*"|'[^']*'|\\.)+`)
var steamProcess = regexp.MustCompile(`(?mi)(?:^|[/\s])(?:steam_osx|Steam\.app/Contents/MacOS/steam)(?:\s|$)`)

func unquoteVDF(s string) string {
	if !strings.HasPrefix(s, "\"") {
		return s
	}
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(s[1 : len(s)-1])
}

func quoteVDF(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func parseVDF(raw string) (vdfEntry, error) {
	var tokens [][2]int
	cursor := 0
	for _, span := range vdfToken.FindAllStringIndex(raw, -1) {
		if strings.TrimSpace(raw[cursor:span[0]]) != "" {
			return vdfEntry{}, errors.New("Steam ayar dosyası okunamadı.")
		}
		cursor = span[1]
		if !strings.HasPrefix(raw[span[0]:span[1]], "//") {
			tokens = append(tokens, [2]int{span[0], span[1]})
		}
	}
	if strings.TrimSpace(raw[cursor:]) != "" {
		return vdfEntry{}, errors.New("Steam ayar dosyası okunamadı.")
	}
	i := 0
	read := func() (string, int, int) {
		t := tokens[i]
		i++
		return raw[t[0]:t[1]], t[0], t[1]
	}
	var object func(int) (vdfEntry, bool)
	object = func(depth int) (vdfEntry, bool) {
		var o vdfEntry
		if depth > 64 {
			return o, false
		}
		for i < len(tokens) {
			key, start, _ := read()
			if key == "}" {
				o.end = start
				return o, depth > 0
			}
			if key == "{" || i == len(tokens) {
				return o, false
			}
			val, start, end := read()
			e := vdfEntry{key: unquoteVDF(key), start: start, end: end}
			switch val {
			case "{":
				child, ok := object(depth + 1)
				if !ok {
					return o, false
				}
				e.children, e.end = child.children, child.end
			case "}":
				return o, false
			default:
				e.value = unquoteVDF(val)
			}
			o.children = append(o.children, e)
		}
		return o, depth == 0
	}
	root, ok := object(0)
	if !ok {
		return vdfEntry{}, errors.New("Steam ayar dosyası okunamadı.")
	}
	return root, nil
}

func steamConfig(home string) (string, error) {
	root := filepath.Join(home, "Library", "Application Support", "Steam")
	entries, _ := os.ReadDir(filepath.Join(root, "userdata"))
	var configs []string
	for _, item := range entries {
		if _, err := strconv.ParseUint(item.Name(), 10, 32); err != nil {
			continue
		}
		config := filepath.Join(root, "userdata", item.Name(), "config", "localconfig.vdf")
		if st, err := os.Stat(config); err == nil && st.Mode().IsRegular() {
			configs = append(configs, config)
		}
	}
	if raw, err := readRegular(filepath.Join(root, "config", "loginusers.vdf")); err == nil {
		if users, err := parseVDF(string(raw)); err == nil {
			for _, user := range users.child("users").children {
				id, err := strconv.ParseUint(user.key, 10, 64)
				if err != nil || user.child("MostRecent").value != "1" {
					continue
				}
				chosen := filepath.Join(root, "userdata", strconv.FormatUint(uint64(uint32(id)), 10), "config", "localconfig.vdf")
				for _, config := range configs {
					if config == chosen {
						return config, nil
					}
				}
				return "", errors.New("Son Steam hesabının yerel ayarları bulunamadı.")
			}
		}
	}
	if len(configs) == 1 {
		return configs[0], nil
	}
	return "", errors.New("Steam hesabı belirlenemedi. Steam’de ETS2 hesabına giriş yap.")
}

func fpsFlags(options string) (sdr, vk, conflict bool, rest string) {
	spans := optionToken.FindAllStringIndex(options, -1)
	last := 0
	var remaining strings.Builder
	for i := 0; i < len(spans); i++ {
		start, end := spans[i][0], spans[i][1]
		flag := strings.Trim(options[start:end], `"'`)
		if flag != "-color_mode" && flag != "-rdevice" {
			continue
		}
		expected := "sdr"
		if flag == "-rdevice" {
			expected = "vk"
		}
		if i+1 == len(spans) || strings.Trim(options[spans[i+1][0]:spans[i+1][1]], `"'`) != expected {
			conflict = true
			continue
		}
		if flag == "-rdevice" {
			vk = true
		} else {
			sdr = true
		}
		i++
		remaining.WriteString(options[last:start])
		last = spans[i][1]
	}
	remaining.WriteString(options[last:])
	return sdr, vk, conflict, strings.TrimSpace(remaining.String())
}

func patchSteamConfig(raw, mode string) (string, bool, error) {
	root, err := parseVDF(raw)
	if err != nil {
		return "", false, err
	}
	app := root.child("UserLocalConfigStore").child("Software").child("Valve").child("Steam").child("apps").child("227300")
	if app.end == 0 || raw[app.start] != '{' {
		return "", false, errors.New("Steam’de ETS2 ayarı bulunamadı. Oyunu Steam’den bir kez aç.")
	}
	opts := app.child("LaunchOptions")
	if opts.start != 0 && raw[opts.start] == '{' {
		return "", false, errors.New("Steam başlatma seçenekleri okunamadı.")
	}
	sdr, vk, conflict, rest := fpsFlags(opts.value)
	enabled := sdr && vk && !conflict
	if mode == "read" {
		return raw, enabled, nil
	}
	if mode == "on" {
		if conflict {
			return "", false, errors.New("Steam’de farklı renk veya render seçeneği var. Önce o seçeneği Steam’den kaldır.")
		}
		if enabled {
			return raw, true, nil
		}
		if rest != "" {
			rest += " "
		}
		rest += fpsOptions
	} else if !sdr && !vk {
		return raw, false, nil
	}
	if opts.start != 0 {
		return raw[:opts.start] + quoteVDF(rest) + raw[opts.end:], mode == "on", nil
	}
	// Insert before the closing brace, leaving the rest of Steam's file intact.
	start := strings.LastIndex(raw[:app.end], "\n") + 1
	indent := raw[start:app.end]
	if strings.TrimSpace(indent) != "" {
		return "", false, errors.New("Steam’de önce ETS2 için bir başlatma seçeneği oluştur.")
	}
	newline := "\n"
	if start >= 2 && raw[start-2] == '\r' {
		newline = "\r\n"
	}
	return raw[:start] + indent + "\t\"LaunchOptions\"\t\t" + quoteVDF(rest) + newline + raw[start:], true, nil
}

func steamCommand(home string, args []string) (steamSettings, error) {
	var state steamSettings
	mode := "read"
	if len(args) == 2 && (args[0] == "on" || args[0] == "off") {
		mode = args[0]
	} else if len(args) != 0 {
		return state, errors.New("Geçersiz Steam ayarı.")
	}
	path, err := steamConfig(home)
	if err != nil {
		return state, err
	}
	raw, err := readRegular(path)
	if err != nil {
		return state, err
	}
	current := steamToken(path, raw)
	running := func() (bool, error) {
		out, err := exec.Command("/bin/ps", "-axo", "ucomm=,comm=").Output()
		return steamProcess.Match(out), err
	}
	state.Running, err = running()
	if err != nil {
		return state, err
	}
	state.Token = current
	if mode != "read" {
		if state.Running {
			return state, errors.New("Bu ayarı kaydetmek için Steam’i tamamen kapat.")
		}
		if args[1] != current {
			return state, errors.New("Steam ayarları değişti. Yenile ve tekrar dene.")
		}
	}
	patched, enabled, err := patchSteamConfig(string(raw), mode)
	state.Enabled = enabled
	if err != nil || patched == string(raw) {
		return state, err
	}
	file, err := stage(path, []byte(patched))
	if err != nil {
		return state, err
	}
	defer os.Remove(file)
	if open, err := running(); err != nil || open {
		return state, errors.New("Steam’in tamamen kapalı olması gerekiyor.")
	}
	now, err := readRegular(path)
	if err != nil || !bytes.Equal(now, raw) {
		return state, errors.New("Steam ayarları değişti. Yenile ve tekrar dene.")
	}
	if err := os.Rename(file, path); err != nil {
		return state, err
	}
	state.Token = steamToken(path, []byte(patched))
	return state, nil
}

func steamToken(path string, raw []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(append([]byte(path+"\x00"), raw...)))
}
