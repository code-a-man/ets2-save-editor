package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/code-a-man/ets2-save-editor/bsii"
	"github.com/code-a-man/ets2-save-editor/decoder"
)

var (
	blockRE     = regexp.MustCompile("(?ms)^([a-zA-Z0-9_]+) : (\\S+) \\{\\r?\\n.*?^\\}\\r?$")
	nameRE      = regexp.MustCompile("_nameless(?:\\.[0-9a-fA-F]+)+")
	gameProcess = regexp.MustCompile("(?mi)(?:^|[/\\s])eurotrucks2(?:\\.exe)?(?:\\s|$)")
	skillKeys   = []string{"adr", "long_dist", "heavy", "fragile", "urgent", "mechanical"}
)

type unit struct {
	kind, id   string
	start, end int
	text       string
}
type document struct {
	text  string
	units []unit
	byID  map[string]unit
}
type slot struct {
	path, token      string
	game, info       document
	rawGame, rawInfo []byte
}
type jobChange struct {
	job, player, vehicles, economy                    unit
	baselinePlayer, baselineVehicles, baselineEconomy unit
}
type saveChoice struct {
	Path     string    `json:"path"`
	Label    string    `json:"label"`
	Modified time.Time `json:"-"`
}

type view struct {
	Saves     []saveChoice `json:"saves"`
	Path      string       `json:"path"`
	Token     string       `json:"token"`
	Money     string       `json:"money"`
	Skills    []int        `json:"skills"`
	Job       string       `json:"job"`
	CanCancel bool         `json:"canCancel"`
	GameOpen  bool         `json:"gameOpen"`
	Backup    string       `json:"backup,omitempty"`
}

func readDoc(raw []byte) (document, error) {
	data, kind, err := decoder.Decode(raw)
	if err != nil {
		return document{}, err
	}
	if kind == decoder.KindBinary {
		data, err = bsii.Decode(data)
		if err != nil {
			return document{}, err
		}
	}
	if !bytes.HasPrefix(data, []byte("SiiNunit")) {
		return document{}, errors.New("SII biçimi tanınmadı.")
	}
	d := document{text: string(data), byID: map[string]unit{}}
	for _, p := range blockRE.FindAllStringSubmatchIndex(d.text, -1) {
		u := unit{d.text[p[2]:p[3]], d.text[p[4]:p[5]], p[0], p[1], d.text[p[0]:p[1]]}
		if _, duplicate := d.byID[u.id]; duplicate {
			return document{}, errors.New("Kayıtta yinelenen birim var.")
		}
		d.units = append(d.units, u)
		d.byID[u.id] = u
	}
	if len(d.units) == 0 {
		return document{}, errors.New("Kayıt okunamadı.")
	}
	return d, nil
}

func (d document) one(kind string) (unit, error) {
	var found unit
	for _, u := range d.units {
		if u.kind != kind {
			continue
		}
		if found.id != "" {
			return unit{}, fmt.Errorf("Birden çok %s alanı var.", kind)
		}
		found = u
	}
	if found.id == "" {
		return unit{}, fmt.Errorf("%s alanı bulunamadı.", kind)
	}
	return found, nil
}

func fieldRE(key string) *regexp.Regexp {
	key = regexp.QuoteMeta(key)
	return regexp.MustCompile("(?m)^([ \\t]+" + key + ":[ \\t]*)([^\\r\\n]*)(?:\\r?\\n[ \\t]+" + key + "\\[\\d+\\]:[^\\r\\n]*)*")
}
func value(u unit, key string) string {
	m := fieldRE(key).FindStringSubmatch(u.text)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[2])
}
func (u *unit) change(key string, replacement func(string, string) string) error {
	m := fieldRE(key).FindAllStringSubmatchIndex(u.text, -1)
	if len(m) != 1 {
		return fmt.Errorf("%s alanı değiştirilemedi.", key)
	}
	p := m[0]
	u.text = u.text[:p[0]] + replacement(u.text[p[0]:p[1]], u.text[p[2]:p[3]]) + u.text[p[1]:]
	return nil
}
func (u *unit) set(key, number string) error {
	return u.change(key, func(_, prefix string) string { return prefix + number })
}
func (u *unit) copyFields(src unit, keys ...string) error {
	for _, key := range keys {
		field := fieldRE(key).FindString(src.text)
		if field == "" {
			return fmt.Errorf("İşsiz kayıtta %s bulunamadı.", key)
		}
		if err := u.change(key, func(_, _ string) string { return field }); err != nil {
			return err
		}
	}
	return nil
}
func (d document) render(changes map[string]string) string {
	var b strings.Builder
	b.Grow(len(d.text))
	last := 0
	for _, u := range d.units {
		if text, ok := changes[u.id]; ok {
			b.WriteString(d.text[last:u.start])
			b.WriteString(text)
			last = u.end
		}
	}
	b.WriteString(d.text[last:])
	return b.String()
}

func readRegular(path string) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > 128*1024*1024 {
		return nil, errors.New("Normal bir ETS2 kayıt dosyası seç.")
	}
	return os.ReadFile(path)
}
func readSlot(path string) (slot, error) {
	if filepath.Base(path) == "game.sii" {
		path = filepath.Dir(path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return slot{}, err
	}
	s := slot{path: abs}
	if _, err := os.Stat(filepath.Join(abs, "game.sii")); os.IsNotExist(err) {
		return s, errors.New("game.sii ile info.sii dosyalarının bulunduğu klasörü seç.")
	}
	if _, err := os.Stat(filepath.Join(abs, "info.sii")); os.IsNotExist(err) {
		return s, errors.New("info.sii, game.sii ile aynı klasörde olmalı.")
	}
	s.rawGame, err = readRegular(filepath.Join(abs, "game.sii"))
	if err != nil {
		return s, err
	}
	s.rawInfo, err = readRegular(filepath.Join(abs, "info.sii"))
	if err != nil {
		return s, err
	}
	s.game, err = readDoc(s.rawGame)
	if err != nil {
		return s, err
	}
	s.info, err = readDoc(s.rawInfo)
	if err != nil {
		return s, err
	}
	s.token = fmt.Sprintf("%x.%x", sha256.Sum256(s.rawGame), sha256.Sum256(s.rawInfo))
	return s, nil
}
func slotLabel(dir, source string) string {
	profile := filepath.Base(filepath.Dir(filepath.Dir(dir)))
	if decoded, err := hex.DecodeString(profile); err == nil && utf8.Valid(decoded) {
		profile = string(decoded)
	}
	name := filepath.Base(dir)
	if name == "autosave" {
		name = "Otomatik kayıt"
	}
	return profile + " · " + name + " · " + source
}

func discoverSaves(home string) []saveChoice {
	support := filepath.Join(home, "Library", "Application Support")
	var saves []saveChoice
	seen := map[string]bool{}
	add := func(root, source string) {
		// Keep the root literal so paths with brackets or other glob syntax work.
		entries, err := os.ReadDir(root)
		if err != nil {
			return
		}
		for _, profile := range entries {
			slots, err := os.ReadDir(filepath.Join(root, profile.Name(), "save"))
			if err != nil {
				continue
			}
			for _, entry := range slots {
				if strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "multiplayer_backup") {
					continue
				}
				dir := filepath.Join(root, profile.Name(), "save", entry.Name())
				resolved, err := filepath.EvalSymlinks(dir)
				if err != nil || seen[resolved] {
					continue
				}
				game, err := os.Lstat(filepath.Join(dir, "game.sii"))
				if err != nil || !game.Mode().IsRegular() {
					continue
				}
				info, err := os.Lstat(filepath.Join(dir, "info.sii"))
				if err != nil || !info.Mode().IsRegular() {
					continue
				}
				seen[resolved] = true
				saves = append(saves, saveChoice{dir, slotLabel(dir, source), game.ModTime()})
			}
		}
	}
	users := filepath.Join(support, "Steam", "userdata")
	accounts, _ := os.ReadDir(users)
	for _, account := range accounts {
		for _, kind := range []string{"profiles", "steam_profiles"} {
			add(filepath.Join(users, account.Name(), "227300", "remote", kind), "Steam Cloud "+account.Name())
		}
	}
	for _, parent := range []string{support, filepath.Join(home, "Documents")} {
		for _, kind := range []string{"profiles", "steam_profiles"} {
			add(filepath.Join(parent, "Euro Truck Simulator 2", kind), "Yerel")
		}
	}
	sort.SliceStable(saves, func(i, j int) bool { return saves[i].Modified.After(saves[j].Modified) })
	for i := range saves {
		saves[i].Label += " · " + saves[i].Modified.Local().Format("02.01.2006 15:04")
	}
	return saves
}

func isGameOpen() (bool, error) {
	data, err := exec.Command("/bin/ps", "-axo", "ucomm=,comm=").Output()
	if err != nil {
		return false, errors.New("Oyunun açık olup olmadığı kontrol edilemedi.")
	}
	return gameProcess.Match(data), nil
}
func (s slot) jobInfo() (string, jobChange) {
	player, err := s.game.one("player")
	if err != nil {
		return "", jobChange{}
	}
	jid := value(player, "current_job")
	if jid == "null" || jid == "" {
		return "", jobChange{}
	}
	job := s.game.byID[jid]
	if job.kind != "player_job" {
		return "Aktif iş", jobChange{}
	}
	cargo := strings.ReplaceAll(strings.TrimPrefix(value(job, "cargo"), "cargo."), "_", " ")
	if cargo == "" {
		cargo = "Aktif iş"
	}
	vehicles := s.game.byID[value(player, "assigned_vehicles")]
	truck, trailer := value(job, "company_truck"), value(job, "company_trailer")
	if value(job, "online_job_id") != "nil" || value(player, "my_vehicles_mode") != "none" ||
		value(player, "assigned_vehicles_mode") != "truck" || vehicles.kind != "player_vehicles" ||
		truck == "null" || trailer == "null" || s.game.byID[truck].kind != "vehicle" ||
		s.game.byID[trailer].kind != "trailer" || value(vehicles, "vehicle") != truck ||
		value(vehicles, "trailer") != trailer {
		return cargo, jobChange{}
	}
	economy, err := s.game.one("economy")
	if err != nil {
		return cargo, jobChange{}
	}
	candidates, _ := filepath.Glob(filepath.Join(filepath.Dir(s.path), "autosave_job*", "game.sii"))
	sort.SliceStable(candidates, func(i, j int) bool {
		a, errA := os.Stat(candidates[i])
		b, errB := os.Stat(candidates[j])
		return errA == nil && (errB != nil || a.ModTime().After(b.ModTime()))
	})
	for _, p := range candidates {
		raw, err := readRegular(p)
		if err != nil {
			continue
		}
		old, err := readDoc(raw)
		if err != nil {
			continue
		}
		op, err := old.one("player")
		if err != nil || value(op, "current_job") != "null" || value(op, "my_vehicles_mode") != "none" ||
			value(op, "assigned_vehicles_mode") != "none" || value(op, "hq_city") != value(player, "hq_city") {
			continue
		}
		ov := old.byID[value(op, "assigned_vehicles")]
		if ov.kind != "player_vehicles" || value(ov, "vehicle") != "null" || value(ov, "trailer") != "null" {
			continue
		}
		oe, err := old.one("economy")
		if err != nil {
			continue
		}
		previousTime, timeErr := strconv.Atoi(value(oe, "game_time"))
		startTime, startErr := strconv.Atoi(value(job, "start_time"))
		if timeErr != nil || startErr != nil || previousTime != startTime {
			continue
		}
		return cargo, jobChange{job, player, vehicles, economy, op, ov, oe}
	}
	return cargo, jobChange{}
}

func refs(text string) []string {
	_, body, _ := strings.Cut(text, "\n")
	return nameRE.FindAllString(body, -1)
}
func (j jobChange) apply(d document, economy unit, changes map[string]string) error {
	p, v := j.player, j.vehicles
	if err := p.copyFields(j.baselinePlayer, "current_job", "assigned_vehicles_mode"); err != nil {
		return err
	}
	if err := v.copyFields(j.baselineVehicles, "vehicle", "stored_vehicle_placement", "trailer",
		"stored_trailer_placements", "stored_trailer_attached"); err != nil {
		return err
	}
	if err := economy.copyFields(j.baselineEconomy, "stored_camera_mode", "stored_actor_state"); err != nil {
		return err
	}
	gps := []string{"stored_gps_behind_waypoints", "stored_gps_ahead_waypoints", "stored_gps_waypoints_history"}
	for _, key := range gps {
		if value(j.baselineEconomy, key) != "0" {
			return errors.New("İşsiz kayıtta başka GPS rotası var.")
		}
		if err := economy.copyFields(j.baselineEconomy, key); err != nil {
			return err
		}
	}
	changes[p.id], changes[v.id], changes[economy.id] = p.text, v.text, economy.text
	discard := map[string]bool{j.job.id: true}
	queue := []string{j.job.id}
	for _, u := range []unit{p, v, economy} {
		retained := map[string]bool{}
		for _, id := range refs(u.text) {
			retained[id] = true
		}
		for _, id := range refs(d.byID[u.id].text) {
			if !retained[id] && !discard[id] {
				discard[id] = true
				queue = append(queue, id)
			}
		}
	}
	for i := 0; i < len(queue); i++ {
		u := d.byID[queue[i]]
		if len(queue) > 500 || u.id == "" {
			return errors.New("İşin araç bağlantıları doğrulanamadı.")
		}
		for _, id := range refs(u.text) {
			if !discard[id] {
				discard[id] = true
				queue = append(queue, id)
			}
		}
	}
	for {
		removed := false
		for _, u := range d.units {
			if discard[u.id] {
				continue
			}
			body := u.text
			if replacement, ok := changes[u.id]; ok {
				body = replacement
			}
			for _, id := range refs(body) {
				if discard[id] {
					delete(discard, id)
					removed = true
				}
			}
		}
		if !removed {
			break
		}
	}
	if !discard[j.job.id] || !discard[value(j.job, "company_truck")] || !discard[value(j.job, "company_trailer")] {
		return errors.New("İşin tırı veya dorsesi başka bir kayıtta kullanılıyor.")
	}
	for id := range discard {
		kind := d.byID[id].kind
		if kind != "player_job" && kind != "vehicle" && kind != "trailer" &&
			kind != "gps_waypoint_storage" && !strings.HasPrefix(kind, "vehicle_") {
			return fmt.Errorf("Beklenmeyen iş bağlantısı: %s", kind)
		}
		changes[id] = ""
	}
	return nil
}

func inspect(s slot) (view, error) {
	economy, err := s.game.one("economy")
	if err != nil {
		return view{}, err
	}
	bank, err := s.game.one("bank")
	if err != nil {
		return view{}, err
	}
	money := value(bank, "money_account")
	if _, err := strconv.ParseInt(money, 10, 64); err != nil {
		return view{}, errors.New("Bakiye okunamadı.")
	}
	v := view{Path: s.path, Token: s.token, Money: money}
	for i, key := range skillKeys {
		n, err := strconv.Atoi(value(economy, key))
		limit := 6
		if i == 0 {
			limit = 63
		}
		if err != nil || n < 0 || n > limit {
			return view{}, fmt.Errorf("%s becerisi okunamadı.", key)
		}
		v.Skills = append(v.Skills, n)
	}
	title, job := s.jobInfo()
	v.Job, v.CanCancel = title, job.job.id != ""
	v.GameOpen, err = isGameOpen()
	return v, err
}

func makeBackup(root string, s slot) (string, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(root, time.Now().Format("2006-01-02_150405")+"_"+filepath.Base(s.path)+"_*.zip")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(f.Name())
		}
	}()
	z := zip.NewWriter(f)
	for _, item := range []struct {
		name string
		data []byte
	}{{"game.sii", s.rawGame}, {"info.sii", s.rawInfo}} {
		w, err := z.Create(filepath.Base(s.path) + "/" + item.name)
		if err != nil {
			return "", err
		}
		if _, err := w.Write(item.data); err != nil {
			return "", err
		}
	}
	if err := z.Close(); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	check, err := zip.OpenReader(f.Name())
	if err != nil {
		return "", err
	}
	defer check.Close()
	if len(check.File) != 2 {
		return "", errors.New("Yedek doğrulanamadı.")
	}
	for i, expected := range [][]byte{s.rawGame, s.rawInfo} {
		entry, err := check.File[i].Open()
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(entry)
		_ = entry.Close()
		if err != nil || !bytes.Equal(data, expected) {
			return "", errors.New("Yedek doğrulanamadı.")
		}
	}
	ok = true
	return f.Name(), nil
}

func stage(target string, data []byte) (string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(target), ".ets2-save-editor-*")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		_ = f.Close()
		if !ok {
			_ = os.Remove(f.Name())
		}
	}()
	if err := f.Chmod(info.Mode().Perm()); err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	ok = true
	return f.Name(), nil
}

func write(s slot, args []string, root string, gameOpen func() (bool, error)) (view, error) {
	if len(args) != 9 {
		return view{}, errors.New("Eksik kayıt ayarı.")
	}
	if args[0] != s.token {
		return view{}, errors.New("Kayıt değişmiş. Yenileyip tekrar dene.")
	}
	open, err := gameOpen()
	if err != nil {
		return view{}, err
	}
	if open {
		return view{}, errors.New("Kaydetmeden önce ETS2’yi kapat.")
	}
	money, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return view{}, errors.New("Bakiye için tam sayı gir.")
	}
	economy, err := s.game.one("economy")
	if err != nil {
		return view{}, err
	}
	bank, err := s.game.one("bank")
	if err != nil {
		return view{}, err
	}
	for i, key := range skillKeys {
		n, err := strconv.Atoi(args[2+i])
		limit := 6
		if i == 0 {
			limit = 63
		}
		if err != nil || n < 0 || n > limit {
			return view{}, fmt.Errorf("Geçersiz beceri: %s", key)
		}
		if strconv.Itoa(n) != value(economy, key) {
			if err := economy.set(key, strconv.Itoa(n)); err != nil {
				return view{}, err
			}
		}
	}
	changes := map[string]string{economy.id: economy.text}
	switch args[8] {
	case "cancel":
		_, job := s.jobInfo()
		if job.job.id == "" {
			return view{}, errors.New("Bu Hızlı İş için uygun bir iş öncesi kayıt bulunamadı.")
		}
		if err := job.apply(s.game, economy, changes); err != nil {
			return view{}, err
		}
	case "keep":
	default:
		return view{}, errors.New("Geçersiz iş seçimi.")
	}
	textInfo := s.info.text
	if amount := strconv.FormatInt(money, 10); amount != value(bank, "money_account") {
		if err := bank.set("money_account", amount); err != nil {
			return view{}, err
		}
		changes[bank.id] = bank.text
		info, err := s.info.one("save_container")
		if err != nil {
			return view{}, err
		}
		if err := info.set("info_money_account", amount); err != nil {
			return view{}, err
		}
		textInfo = s.info.render(map[string]string{info.id: info.text})
	}
	textGame := s.game.render(changes)
	if textGame == s.game.text && textInfo == s.info.text {
		return view{}, errors.New("Kaydedilecek değişiklik yok.")
	}
	backup, err := makeBackup(root, s)
	if err != nil {
		return view{}, err
	}
	gp, ip := filepath.Join(s.path, "game.sii"), filepath.Join(s.path, "info.sii")
	gameStage, err := stage(gp, []byte(textGame))
	if err != nil {
		return view{}, err
	}
	defer os.Remove(gameStage)
	infoStage, rollback := "", ""
	if textInfo != s.info.text {
		infoStage, err = stage(ip, []byte(textInfo))
		if err != nil {
			return view{}, err
		}
		defer os.Remove(infoStage)
		rollback, err = stage(gp, s.rawGame)
		if err != nil {
			return view{}, err
		}
		defer os.Remove(rollback)
	}
	now, err := readSlot(s.path)
	if err != nil || now.token != s.token {
		return view{}, errors.New("Kayıt değişmiş. Yenileyip tekrar dene.")
	}
	open, err = gameOpen()
	if err != nil {
		return view{}, err
	}
	if open {
		return view{}, errors.New("Kaydetmeden önce ETS2’yi kapat.")
	}
	if err := os.Rename(gameStage, gp); err != nil {
		return view{}, err
	}
	if infoStage != "" {
		if err := os.Rename(infoStage, ip); err != nil {
			if rollbackErr := os.Rename(rollback, gp); rollbackErr != nil {
				return view{}, fmt.Errorf("Kayıt geri alınamadı. Yedek: %s", backup)
			}
			return view{}, err
		}
	}
	updated, err := readSlot(s.path)
	if err != nil {
		return view{}, err
	}
	v, err := inspect(updated)
	v.Backup = backup
	return v, err
}

func run(args []string) (view, error) {
	if len(args) == 0 || (args[0] != "read" && args[0] != "write") {
		return view{}, errors.New("Geçersiz komut.")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return view{}, err
	}
	choices := discoverSaves(home)
	if choices == nil {
		choices = []saveChoice{}
	}

	if args[0] == "read" && len(args) == 1 {
		for _, choice := range choices {
			s, err := readSlot(choice.Path)
			if err != nil {
				continue
			}
			v, err := inspect(s)
			if err == nil {
				v.Saves = choices
				return v, nil
			}
		}
		if len(choices) == 0 {
			return view{}, errors.New("ETS2 kaydı bulunamadı. Seç… ile kayıt klasörünü aç.")
		}
		return view{}, errors.New("Otomatik bulunan kayıtlar açılamadı. Seç… ile başka bir kayıt aç.")
	}
	if len(args) < 2 {
		return view{}, errors.New("Kayıt seçilmedi.")
	}
	s, err := readSlot(args[1])
	if err != nil {
		return view{}, err
	}

	var v view
	if args[0] == "write" {
		root := filepath.Join(home, "Library", "Application Support", "ETS2 Save Editor", "Yedekler")
		v, err = write(s, args[2:], root, isGameOpen)
		if err == nil {
			choices = discoverSaves(home)
		}
	} else {
		v, err = inspect(s)
	}
	if choices == nil {
		choices = []saveChoice{}
	}
	v.Saves = choices
	return v, err
}
func main() {
	v, err := run(os.Args[1:])
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(v)
}
