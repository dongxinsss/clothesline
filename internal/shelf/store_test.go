package shelf

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func pngBytes(t *testing.T) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 800, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 800; x++ {
			im.Set(x, y, color.RGBA{uint8(x), uint8(y), 100, 255})
		}
	}
	var b bytes.Buffer
	if e := png.Encode(&b, im); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestPersistenceDedupAndChronologicalUndo(t *testing.T) {
	s := newStore(t)
	first, added, e := s.AddText("  首条便签  ", "便签")
	if e != nil || !added {
		t.Fatalf("%v %v", added, e)
	}
	if _, added, e = s.AddText("首条便签", "剪贴板"); e != nil || added {
		t.Fatalf("duplicate: %v %v", added, e)
	}
	second, _, _ := s.AddText("第二条", "便签")
	if e = s.Remove(first.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.Restore(first); e != nil {
		t.Fatal(e)
	}
	reopened, e := Open(s.dir)
	if e != nil {
		t.Fatal(e)
	}
	items := reopened.Snapshot().Items
	if len(items) != 2 || items[0].ID != first.ID || items[1].ID != second.ID || !items[0].CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("wrong restored order: %#v", items)
	}
}
func TestImageThumbnailAndValidation(t *testing.T) {
	s := newStore(t)
	data := pngBytes(t)
	i, added, e := s.AddImage(data, "导入")
	if e != nil || !added {
		t.Fatalf("%v %v", added, e)
	}
	if _, added, e = s.AddImage(data, "剪贴板"); e != nil || added {
		t.Fatalf("duplicate: %v %v", added, e)
	}
	p, e := s.ImagePath(i.ID, true)
	if e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	cfg, _, e := image.DecodeConfig(f)
	if e != nil || cfg.Width > 400 || cfg.Height > 280 {
		t.Fatalf("thumbnail: %#v %v", cfg, e)
	}
	if _, e = s.ImagePath("../../private", false); e == nil {
		t.Fatal("accepted unregistered path")
	}
	if _, _, e = s.AddImage([]byte("not an image"), "导入"); e == nil {
		t.Fatal("accepted invalid image")
	}
	if i.Width != 800 || i.Height != 400 {
		t.Fatal("wrong dimensions")
	}
}
func TestFailedPersistenceDoesNotChangeMemory(t *testing.T) {
	s := newStore(t)
	if e := os.Mkdir(filepath.Join(s.dir, "shelf.json.bak"), 0700); e != nil {
		t.Fatal(e)
	}
	if _, _, e := s.AddText("cannot persist", "便签"); e == nil {
		t.Fatal("expected write failure")
	}
	if len(s.Snapshot().Items) != 0 {
		t.Fatal("unpersisted item leaked into memory")
	}
	old := s.Snapshot().Settings
	if e := s.UpdateSettings(Settings{AutoCollect: false, AlwaysOnTop: false, HeightPercent: 25}); e == nil {
		t.Fatal("expected settings write failure")
	}
	if s.Snapshot().Settings != old {
		t.Fatal("settings not rolled back")
	}
}
func TestCorruptManifestIsPreserved(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "shelf.json")
	bad := []byte("{broken")
	os.WriteFile(p, bad, 0600)
	if _, e := Open(d); e == nil {
		t.Fatal("expected corrupt manifest error")
	}
	b, _ := os.ReadFile(p)
	if !bytes.Equal(b, bad) {
		t.Fatal("corrupt manifest overwritten")
	}
}
func TestSettingsAndSnapshotIsolation(t *testing.T) {
	s := newStore(t)
	s.AddText("original", "便签")
	state := s.Snapshot()
	state.Items[0].Text = "changed"
	if s.Snapshot().Items[0].Text != "original" {
		t.Fatal("snapshot aliases store")
	}
	if e := s.UpdateSettings(Settings{AutoCollect: true, AlwaysOnTop: true, HeightPercent: 99}); e == nil {
		t.Fatal("invalid height accepted")
	}
	if e := s.UpdateSettings(Settings{AutoCollect: false, AlwaysOnTop: false, HeightPercent: 15}); e != nil {
		t.Fatal(e)
	}
	s2, e := Open(s.dir)
	if e != nil {
		t.Fatal(e)
	}
	if s2.Snapshot().Settings.AutoCollect || s2.Snapshot().Settings.HeightPercent != 15 {
		t.Fatal("settings not persisted")
	}
}
