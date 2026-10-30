package shelf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const MaxImageBytes = 32 << 20

type Item struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text,omitempty"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
	Width     int       `json:"width,omitempty"`
	Height    int       `json:"height,omitempty"`
}
type Settings struct {
	AutoCollect   bool   `json:"autoCollect"`
	AlwaysOnTop   bool   `json:"alwaysOnTop"`
	HeightPercent int    `json:"heightPercent"`
	Theme         string `json:"theme"`
	FontSize      int    `json:"fontSize"`
	NoteFontSize  int    `json:"noteFontSize"`
	MetaFontSize  int    `json:"metaFontSize"`
	CardWidth     int    `json:"cardWidth"`
	ShowDockIcon  bool   `json:"showDockIcon"`
	IdleSeconds   int    `json:"idleSeconds"`
	// Inverted so records written before hotkeys existed keep them enabled.
	DisableHotkeys bool `json:"disableHotkeys"`
}
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Frame is the shelf's position and size after the user moved or resized it,
// relative to the work area of its screen.
type Frame struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}
type State struct {
	Items     []Item     `json:"items"`
	History   []Item     `json:"history"`
	Schedules []Schedule `json:"schedules"`
	Settings  Settings   `json:"settings"`
	Basket    *Point     `json:"basket,omitempty"`
	Shelf     *Frame     `json:"shelf,omitempty"`
	// HistoryCount stands in for History in change events, which omit it.
	HistoryCount int    `json:"historyCount"`
	DataDir      string `json:"dataDir,omitempty"`
	Warning      string `json:"warning,omitempty"`
}

func DefaultSettings() Settings {
	return Settings{AutoCollect: true, AlwaysOnTop: true, HeightPercent: 20, Theme: "blue", FontSize: 14, NoteFontSize: 15, MetaFontSize: 11, CardWidth: 180, ShowDockIcon: true}
}
func normalizeSettings(v Settings) Settings {
	if v.HeightPercent == 0 {
		v.HeightPercent = 20
	}
	if v.Theme == "" {
		v.Theme = "blue"
	}
	if v.FontSize == 0 {
		v.FontSize = 14
	}
	if v.NoteFontSize == 0 {
		v.NoteFontSize = 15
	}
	if v.MetaFontSize == 0 {
		v.MetaFontSize = 11
	}
	if v.CardWidth == 0 {
		v.CardWidth = 180
	}
	return v
}
func validID(id string) bool {
	if len(id) != 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil && id == strings.ToLower(id)
}
func validateState(v State) error {
	ids := map[string]bool{}
	for _, items := range [][]Item{v.Items, v.History} {
		for _, i := range items {
			if !validID(i.ID) || (i.Kind != "image" && i.Kind != "text") || ids[i.ID] {
				return errors.New("收纳记录包含无效或重复条目")
			}
			ids[i.ID] = true
		}
	}
	scheduleIDs := map[string]bool{}
	for _, entry := range v.Schedules {
		_, idErr := hex.DecodeString(entry.ID)
		if len(entry.ID) != 32 || idErr != nil || scheduleIDs[entry.ID] {
			return errors.New("日程记录包含无效或重复条目")
		}
		scheduleIDs[entry.ID] = true
		if _, err := validateSchedule(ScheduleDraft{Title: entry.Title, StartDate: entry.StartDate, EndDate: entry.EndDate, AtTime: entry.AtTime, Notes: entry.Notes}); err != nil {
			return err
		}
	}
	return nil
}

type Store struct {
	mu      sync.Mutex
	dir     string
	state   State
	warning string
	// raw maps hashes of image bytes as received to item IDs, so repeated
	// clipboard reads skip decoding and re-encoding.
	raw map[string]string
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, state: State{Items: []Item{}, History: []Item{}, Settings: DefaultSettings()}, raw: map[string]string{}}
	b, err := os.ReadFile(filepath.Join(dir, "shelf.json"))
	recoveredBackup := false
	if err == nil {
		if err = json.Unmarshal(b, &s.state); err == nil {
			err = validateState(s.state)
		}
		if err != nil {
			backup, backupErr := os.ReadFile(filepath.Join(dir, "shelf.json.bak"))
			recovered := State{Settings: DefaultSettings()}
			if backupErr != nil || json.Unmarshal(backup, &recovered) != nil || validateState(recovered) != nil {
				return nil, fmt.Errorf("收纳记录无法读取，请保留 shelf.json 并恢复备份：%w", err)
			}
			preserved := filepath.Join(dir, "shelf.corrupt-"+time.Now().Format("20060102-150405.000000000")+".json")
			if e := os.WriteFile(preserved, b, 0600); e != nil {
				return nil, e
			}
			s.state = recovered
			s.warning = "主记录损坏，已读取上次备份；损坏文件已另行保留"
			recoveredBackup = true
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if s.state.Items == nil {
		s.state.Items = []Item{}
	}
	if s.state.History == nil {
		s.state.History = []Item{}
	}
	if s.state.Schedules == nil {
		s.state.Schedules = []Schedule{}
	}
	s.state.Settings = normalizeSettings(s.state.Settings)
	if s.state.Settings.HeightPercent < 12 || s.state.Settings.HeightPercent > 30 {
		s.state.Settings.HeightPercent = 20
	}
	s.state.Warning = ""
	s.state.HistoryCount = 0
	if errors.Is(err, os.ErrNotExist) || recoveredBackup {
		if e := s.persist(); e != nil {
			return nil, e
		}
	}
	return s, nil
}
func (s *Store) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.state
	out.Items = append([]Item{}, s.state.Items...)
	out.History = append([]Item{}, s.state.History...)
	out.Schedules = append([]Schedule{}, s.state.Schedules...)
	out.DataDir = s.dir
	out.Warning = s.warning
	out.HistoryCount = len(s.state.History)
	if s.state.Basket != nil {
		p := *s.state.Basket
		out.Basket = &p
	}
	if s.state.Shelf != nil {
		f := *s.state.Shelf
		out.Shelf = &f
	}
	return out
}

// Summary is Snapshot without the history list, for frequent change events.
func (s *Store) Summary() State {
	out := s.Snapshot()
	out.History = nil
	return out
}
func (s *Store) History() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Item{}, s.state.History...)
}
func (s *Store) SetBasket(p Point) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Basket != nil && *s.state.Basket == p {
		return nil
	}
	old := s.state.Basket
	s.state.Basket = &p
	if err := s.persist(); err != nil {
		s.state.Basket = old
		return err
	}
	return nil
}

// SetShelf saves the shelf frame; nil goes back to the default layout.
func (s *Store) SetShelf(f *Frame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.state.Shelf
	if (old == nil && f == nil) || (old != nil && f != nil && *old == *f) {
		return nil
	}
	if f != nil {
		c := *f
		f = &c
	}
	s.state.Shelf = f
	if err := s.persist(); err != nil {
		s.state.Shelf = old
		return err
	}
	return nil
}
func (s *Store) Dir() string { return s.dir }
func (s *Store) persist() error {
	file := filepath.Join(s.dir, "shelf.json")
	if old, err := os.ReadFile(file); err == nil && json.Valid(old) {
		if err = atomicWrite(s.dir, "shelf.json.bak", old); err != nil {
			return err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.dir, "shelf.json", b)
}
func atomicWrite(dir, name string, b []byte) error {
	f, err := os.CreateTemp(dir, ".shelf-*")
	if err != nil {
		return err
	}
	tmpName := f.Name()
	defer os.Remove(tmpName)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmpName, filepath.Join(dir, name))
}
func (s *Store) UpdateSettings(v Settings) error {
	v = normalizeSettings(v)
	if v.IdleSeconds < 0 || v.IdleSeconds > 86400 {
		return errors.New("自动收起时间需要为 0（永不）或 1 到 86400 秒")
	}
	if v.HeightPercent < 12 || v.HeightPercent > 30 {
		return errors.New("高度必须在 12% 到 30% 之间")
	}
	if v.Theme != "blue" && v.Theme != "dark" && v.Theme != "paper" && v.Theme != "contrast" {
		return errors.New("请选择有效主题")
	}
	if v.FontSize < 12 || v.FontSize > 20 || v.NoteFontSize < 12 || v.NoteFontSize > 26 || v.MetaFontSize < 10 || v.MetaFontSize > 16 || v.CardWidth < 140 || v.CardWidth > 260 {
		return errors.New("字号或卡片宽度超出允许范围")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.state.Settings
	s.state.Settings = v
	if err := s.persist(); err != nil {
		s.state.Settings = old
		return err
	}
	return nil
}
func hash(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func (s *Store) AddText(text, source string) (Item, bool, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Item{}, false, errors.New("先写一点文字吧")
	}
	if len([]rune(text)) > 20000 {
		return Item{}, false, errors.New("便签最多 20000 字")
	}
	i := Item{ID: hash([]byte("text:" + text)), Kind: "text", Text: text, Source: source, CreatedAt: time.Now()}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.add(i)
}
func (s *Store) add(i Item) (Item, bool, error) {
	for _, old := range s.state.Items {
		if old.ID == i.ID {
			return old, false, nil
		}
	}
	for _, old := range s.state.History {
		if old.ID == i.ID {
			err := s.restoreLocked(old)
			return old, err == nil, err
		}
	}
	if len(s.state.Items) >= 1000 {
		return Item{}, false, errors.New("已收纳 1000 项，请先移除不需要的卡片")
	}
	s.state.Items = append(s.state.Items, i)
	sort.SliceStable(s.state.Items, func(a, b int) bool { return s.state.Items[a].CreatedAt.Before(s.state.Items[b].CreatedAt) })
	if err := s.persist(); err != nil {
		for n, v := range s.state.Items {
			if v.ID == i.ID {
				s.state.Items = append(s.state.Items[:n], s.state.Items[n+1:]...)
				break
			}
		}
		return Item{}, false, err
	}
	return i, true, nil
}
func (s *Store) AddImage(data []byte, source string) (Item, bool, error) {
	if len(data) > MaxImageBytes {
		return Item{}, false, errors.New("图片不能超过 32 MB")
	}
	key := hash(data)
	s.mu.Lock()
	id := s.raw[key]
	if id == "" {
		id = key // stored files are normalized, so their bytes hash to the ID
	}
	if old, ok := s.findLocked(id); ok {
		defer s.mu.Unlock()
		return s.add(old)
	}
	s.mu.Unlock()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Item{}, false, errors.New("支持 PNG、JPEG、GIF、WebP 图片")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 50_000_000 {
		return Item{}, false, errors.New("图片尺寸过大，最多 5000 万像素")
	}
	im, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Item{}, false, err
	}
	// The thumbnail does not depend on the normalized PNG; build both at once.
	thumbDone := make(chan error, 1)
	var tb bytes.Buffer
	go func() {
		w, h := cfg.Width, cfg.Height
		if w > 400 || h > 280 {
			scale := min(400.0/float64(w), 280.0/float64(h))
			w = max(1, int(float64(w)*scale))
			h = max(1, int(float64(h)*scale))
		}
		thumb := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(thumb, thumb.Bounds(), im, im.Bounds(), draw.Over, nil)
		thumbDone <- jpeg.Encode(&tb, thumb, &jpeg.Options{Quality: 82})
	}()
	var normalized bytes.Buffer
	err = png.Encode(&normalized, im)
	if thumbErr := <-thumbDone; err == nil {
		err = thumbErr
	}
	if err != nil {
		return Item{}, false, err
	}
	i := Item{ID: hash(normalized.Bytes()), Kind: "image", Source: source, CreatedAt: time.Now(), Width: cfg.Width, Height: cfg.Height}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.raw) > 4096 {
		s.raw = map[string]string{}
	}
	s.raw[key] = i.ID
	if old, ok := s.findLocked(i.ID); ok {
		return s.add(old)
	}
	if len(s.state.Items) >= 1000 {
		return Item{}, false, errors.New("已收纳 1000 项，请先移除不需要的卡片")
	}
	if err = os.WriteFile(filepath.Join(s.dir, i.ID+".png"), normalized.Bytes(), 0600); err != nil {
		return Item{}, false, err
	}
	if err = os.WriteFile(filepath.Join(s.dir, i.ID+".jpg"), tb.Bytes(), 0600); err != nil {
		return Item{}, false, err
	}
	return s.add(i)
}
func (s *Store) findLocked(id string) (Item, bool) {
	for _, items := range [][]Item{s.state.Items, s.state.History} {
		for _, i := range items {
			if i.ID == id {
				return i, true
			}
		}
	}
	return Item{}, false
}

// Purge permanently deletes archived records and their image files. Records
// still on the shelf are left alone.
func (s *Store) Purge(ids []string) (int, error) {
	drop := map[string]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.state.History
	kept, gone := []Item{}, []Item{}
	for _, i := range old {
		if drop[i.ID] {
			gone = append(gone, i)
		} else {
			kept = append(kept, i)
		}
	}
	if len(gone) == 0 {
		return 0, nil
	}
	s.state.History = kept
	if err := s.persist(); err != nil {
		s.state.History = old
		return 0, err
	}
	for _, i := range gone {
		if i.Kind == "image" {
			os.Remove(filepath.Join(s.dir, i.ID+".png"))
			os.Remove(filepath.Join(s.dir, i.ID+".jpg"))
		}
	}
	return len(gone), nil
}
func (s *Store) Find(id string) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i, ok := s.findLocked(id); ok {
		return i, nil
	}
	return Item{}, errors.New("卡片不存在")
}
func (s *Store) ImagePath(id string, thumb bool) (string, error) {
	i, err := s.Find(id)
	if err != nil || i.Kind != "image" {
		return "", errors.New("图片不存在")
	}
	ext := ".png"
	if thumb {
		ext = ".jpg"
	}
	return filepath.Join(s.dir, i.ID+ext), nil
}
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for n, i := range s.state.Items {
		if i.ID == id {
			old := s.state.Items
			oldHistory := s.state.History
			s.state.Items = append(append([]Item{}, old[:n]...), old[n+1:]...)
			s.state.History = append(append([]Item{}, oldHistory...), i)
			if err := s.persist(); err != nil {
				s.state.Items = old
				s.state.History = oldHistory
				return err
			}
			return nil
		}
	}
	return errors.New("卡片不存在")
}

// Restore preserves the original timestamp and chronological position.
func (s *Store) Restore(i Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restoreLocked(i)
}
func (s *Store) restoreLocked(i Item) error {
	for _, old := range s.state.Items {
		if old.ID == i.ID {
			return nil
		}
	}
	if len(s.state.Items) >= 1000 {
		return errors.New("挂绳已有 1000 项，请先移入历史记录后再挂上")
	}
	old := s.state.Items
	oldHistory := s.state.History
	s.state.History = []Item{}
	for _, v := range oldHistory {
		if v.ID != i.ID {
			s.state.History = append(s.state.History, v)
		}
	}
	items := append([]Item{}, old...)
	pos := len(items)
	for n, v := range items {
		if v.CreatedAt.After(i.CreatedAt) {
			pos = n
			break
		}
	}
	items = append(items, Item{})
	copy(items[pos+1:], items[pos:])
	items[pos] = i
	s.state.Items = items
	if err := s.persist(); err != nil {
		s.state.Items = old
		s.state.History = oldHistory
		return err
	}
	return nil
}

// MergeFrom imports old local records without changing or removing the source.
func (s *Store) MergeFrom(dir string) (int, error) {
	if filepath.Clean(dir) == filepath.Clean(s.dir) {
		return 0, nil
	}
	b, err := os.ReadFile(filepath.Join(dir, "shelf.json"))
	if err != nil {
		return 0, err
	}
	old := State{}
	if err = json.Unmarshal(b, &old); err != nil {
		return 0, err
	}
	if err = validateState(old); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	beforeItems, beforeHistory := s.state.Items, s.state.History
	beforeSchedules := s.state.Schedules
	known := map[string]bool{}
	for _, list := range [][]Item{beforeItems, beforeHistory} {
		for _, i := range list {
			known[i.ID] = true
		}
	}
	items, history := append([]Item{}, beforeItems...), append([]Item{}, beforeHistory...)
	added := 0
	for n, list := range [][]Item{old.Items, old.History} {
		for _, i := range list {
			if known[i.ID] {
				continue
			}
			if i.Kind == "image" {
				for _, ext := range []string{".png", ".jpg"} {
					p := filepath.Join(dir, i.ID+ext)
					info, e := os.Lstat(p)
					if e != nil || !info.Mode().IsRegular() || info.Size() > MaxImageBytes {
						return 0, errors.New("旧记录中的图片缺失或无效，原记录已保留")
					}
					data, e := os.ReadFile(p)
					if e != nil {
						return 0, e
					}
					if e = os.WriteFile(filepath.Join(s.dir, i.ID+ext), data, 0600); e != nil {
						return 0, e
					}
				}
			}
			if n == 0 {
				items = append(items, i)
			} else {
				history = append(history, i)
			}
			known[i.ID] = true
			added++
		}
	}
	schedules := append([]Schedule{}, beforeSchedules...)
	scheduleIDs := map[string]bool{}
	for _, v := range schedules {
		scheduleIDs[v.ID] = true
	}
	for _, v := range old.Schedules {
		if !scheduleIDs[v.ID] {
			schedules = append(schedules, v)
			scheduleIDs[v.ID] = true
			added++
		}
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].CreatedAt.Before(items[b].CreatedAt) })
	s.state.Items, s.state.History, s.state.Schedules = items, history, schedules
	if err = s.persist(); err != nil {
		s.state.Items, s.state.History = beforeItems, beforeHistory
		s.state.Schedules = beforeSchedules
		return 0, err
	}
	return added, nil
}
