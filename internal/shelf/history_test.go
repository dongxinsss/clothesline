package shelf

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestArchivedRecordsSurviveRestartAndCanBeRestored(t *testing.T) {
	s := newStore(t)
	i, _, _ := s.AddText("历史记录重启后仍在", "剪贴板")
	if err := s.Remove(i.ID); err != nil {
		t.Fatal(err)
	}
	s, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if len(snap.Items) != 0 || len(snap.History) != 1 {
		t.Fatalf("archive missing: active=%d history=%d", len(snap.Items), len(snap.History))
	}
	if _, err = s.Find(i.ID); err != nil {
		t.Fatal("archived content cannot be previewed")
	}
	if err = s.Restore(i); err != nil {
		t.Fatal(err)
	}
	s, err = Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Snapshot().Items) != 1 || len(s.Snapshot().History) != 0 {
		t.Fatal("restore was not persisted")
	}
}
func TestRepastingArchivedContentDoesNotCreateDuplicateIDs(t *testing.T) {
	s := newStore(t)
	i, _, _ := s.AddText("same text", "便签")
	s.Remove(i.ID)
	_, added, err := s.AddText("same text", "剪贴板")
	if err != nil || !added {
		t.Fatalf("repaste: %v %v", added, err)
	}
	if _, err = Open(s.dir); err != nil {
		t.Fatal(err)
	}
	if len(s.Snapshot().Items) != 1 || len(s.Snapshot().History) != 0 {
		t.Fatal("repaste duplicated archive")
	}
}
func TestAppearanceSettingsSurviveRestartAndLegacyUpgrade(t *testing.T) {
	s := newStore(t)
	v := DefaultSettings()
	v.Theme = "dark"
	v.FontSize = 18
	v.NoteFontSize = 22
	v.MetaFontSize = 14
	v.CardWidth = 220
	v.ShowDockIcon = false
	v.IdleSeconds = 37
	if err := s.UpdateSettings(v); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Snapshot().Settings != v {
		t.Fatal("appearance settings lost on restart")
	}
	d := t.TempDir()
	legacy := []byte(`{"items":[],"settings":{"autoCollect":false,"alwaysOnTop":true,"heightPercent":25}}`)
	os.WriteFile(filepath.Join(d, "shelf.json"), legacy, 0600)
	s3, err := Open(d)
	if err != nil {
		t.Fatal(err)
	}
	got := s3.Snapshot().Settings
	if got.Theme != "blue" || got.FontSize != 14 || !got.ShowDockIcon || got.AutoCollect || got.HeightPercent != 25 {
		t.Fatalf("bad legacy defaults: %#v", got)
	}
	bad := v
	bad.Theme = "unknown"
	if err = s.UpdateSettings(bad); err == nil {
		t.Fatal("unknown theme accepted")
	}
	bad = v
	bad.FontSize = 50
	if err = s.UpdateSettings(bad); err == nil {
		t.Fatal("invalid font size accepted")
	}
	bad = v
	bad.IdleSeconds = -1
	if err = s.UpdateSettings(bad); err == nil {
		t.Fatal("negative idle timeout accepted")
	}
}
func TestLegacyMergeIsIdempotentAndPreservesSourceAndDestination(t *testing.T) {
	old := newStore(t)
	old.AddText("old active", "剪贴板")
	archived, _, _ := old.AddText("old archived", "便签")
	old.Remove(archived.ID)
	old.AddImage(pngBytes(t), "导入")
	p := filepath.Join(old.dir, "shelf.json")
	before, _ := os.ReadFile(p)
	dest := newStore(t)
	dest.AddText("current item", "便签")
	settings := DefaultSettings()
	settings.Theme = "paper"
	dest.UpdateSettings(settings)
	n, err := dest.MergeFrom(old.dir)
	if err != nil || n != 3 {
		t.Fatalf("merge count=%d err=%v", n, err)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("source was modified")
	}
	n, err = dest.MergeFrom(old.dir)
	if err != nil || n != 0 {
		t.Fatalf("merge not idempotent: %d %v", n, err)
	}
	reopened, err := Open(dest.dir)
	if err != nil {
		t.Fatal(err)
	}
	snap := reopened.Snapshot()
	if len(snap.Items) != 3 || len(snap.History) != 1 || snap.Settings.Theme != "paper" {
		t.Fatal("merge overwrote current data or settings")
	}
}
func TestBackupRecoveryPreservesDamagedManifest(t *testing.T) {
	s := newStore(t)
	s.AddText("first persisted record", "便签")
	s.AddText("second persisted record", "便签")
	bad := []byte("{corrupt")
	os.WriteFile(filepath.Join(s.dir, "shelf.json"), bad, 0600)
	recovered, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered.Snapshot().Items) != 1 || recovered.Snapshot().Warning == "" {
		t.Fatal("backup not recovered or warning missing")
	}
	files, _ := filepath.Glob(filepath.Join(s.dir, "shelf.corrupt-*.json"))
	if len(files) != 1 {
		t.Fatal("damaged original not preserved")
	}
	data, _ := os.ReadFile(files[0])
	if !bytes.Equal(data, bad) {
		t.Fatal("wrong damaged backup")
	}
}
func TestInitialManifestAndBackupArePersisted(t *testing.T) {
	s := newStore(t)
	p := filepath.Join(s.dir, "shelf.json")
	b, err := os.ReadFile(p)
	if err != nil || !json.Valid(b) {
		t.Fatal("fresh store did not persist initial state")
	}
	s.AddText("save", "便签")
	b, err = os.ReadFile(p + ".bak")
	if err != nil || !json.Valid(b) {
		t.Fatal("valid backup was not written")
	}
}
func TestRepeatedImageBytesSkipDecodingAndStoredBytesMatchID(t *testing.T) {
	s := newStore(t)
	data := pngBytes(t)
	i, added, err := s.AddImage(data, "剪贴板")
	if err != nil || !added {
		t.Fatal(added, err)
	}
	stored, err := os.ReadFile(filepath.Join(s.Dir(), i.ID+".png"))
	if err != nil {
		t.Fatal(err)
	}
	// Copying a card back out writes the stored bytes; they must dedupe instantly.
	for _, b := range [][]byte{data, stored} {
		again, added, err := s.AddImage(b, "剪贴板")
		if err != nil || added || again.ID != i.ID {
			t.Fatalf("duplicate handling: %v %v %v", again.ID, added, err)
		}
	}
	if err = s.Remove(i.ID); err != nil {
		t.Fatal(err)
	}
	if _, added, _ = s.AddImage(stored, "剪贴板"); !added || len(s.Snapshot().Items) != 1 {
		t.Fatal("re-adding archived bytes should restore the record")
	}
}
func TestPurgeDeletesOnlyArchivedRecordsAndFiles(t *testing.T) {
	s := newStore(t)
	img, _, _ := s.AddImage(pngBytes(t), "导入")
	text, _, _ := s.AddText("留着", "便签")
	if n, err := s.Purge([]string{img.ID, text.ID}); err != nil || n != 0 {
		t.Fatalf("shelf records purged: %d %v", n, err)
	}
	s.Remove(img.ID)
	if n, err := s.Purge([]string{img.ID}); err != nil || n != 1 {
		t.Fatalf("purge: %d %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), img.ID+".png")); !os.IsNotExist(err) {
		t.Fatal("image file kept after purge")
	}
	reopened, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	snap := reopened.Snapshot()
	if len(snap.History) != 0 || len(snap.Items) != 1 || snap.HistoryCount != 0 {
		t.Fatalf("after purge: %#v", snap)
	}
}
func TestBasketPositionSurvivesRestartAndSummaryOmitsHistory(t *testing.T) {
	s := newStore(t)
	if err := s.SetBasket(Point{X: 40, Y: 500}); err != nil {
		t.Fatal(err)
	}
	t1, _, _ := s.AddText("一", "便签")
	s.Remove(t1.ID)
	reopened, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	sum := reopened.Summary()
	if sum.Basket == nil || *sum.Basket != (Point{X: 40, Y: 500}) || sum.History != nil || sum.HistoryCount != 1 {
		t.Fatalf("summary: %#v", sum)
	}
	if len(reopened.History()) != 1 {
		t.Fatal("history lost")
	}
}

func TestShelfFrameSurvivesRestartAndClears(t *testing.T) {
	s := newStore(t)
	if err := s.SetShelf(&Frame{X: 30, Y: 10, W: 900, H: 260}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	snap := reopened.Snapshot()
	if snap.Shelf == nil || *snap.Shelf != (Frame{X: 30, Y: 10, W: 900, H: 260}) {
		t.Fatalf("shelf: %#v", snap.Shelf)
	}
	snap.Shelf.W = 1
	if reopened.Snapshot().Shelf.W != 900 {
		t.Fatal("snapshot shares the frame")
	}
	if err := reopened.SetShelf(nil); err != nil {
		t.Fatal(err)
	}
	if again, _ := Open(s.Dir()); again.Snapshot().Shelf != nil {
		t.Fatal("frame not cleared")
	}
}
