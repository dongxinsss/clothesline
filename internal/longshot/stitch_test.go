package longshot

import (
	"errors"
	"image"
	"image/color"
	"testing"
)

func page(w, h int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint32(y*92821 + x*68917 + 1)
			v ^= v << 13
			v ^= v >> 17
			v ^= v << 5
			im.SetRGBA(x, y, color.RGBA{uint8(v), uint8(v >> 8), uint8(v >> 16), 255})
		}
	}
	return im
}
func tile(t *testing.T, im image.Image, y, h int) *image.RGBA {
	t.Helper()
	out, e := CopyRegion(im, image.Rect(0, y, im.Bounds().Dx(), y+h))
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func TestAutoOverlapStitchesExactPixels(t *testing.T) {
	full := page(180, 800)
	first := tile(t, full, 0, 280)
	next := tile(t, full, 160, 280)
	third := tile(t, full, 340, 280)
	s, e := New(first)
	if e != nil {
		t.Fatal(e)
	}
	status, e := s.Add(next, -1)
	if e != nil || status.Overlap != 120 {
		t.Fatalf("overlap=%d err=%v", status.Overlap, e)
	}
	status, e = s.Add(third, -1)
	if e != nil || status.Overlap != 100 {
		t.Fatalf("overlap=%d err=%v", status.Overlap, e)
	}
	out := s.Image()
	if out.Bounds().Dy() != 620 {
		t.Fatalf("wrong output height %d", out.Bounds().Dy())
	}
	for y := 0; y < 620; y++ {
		for x := 0; x < 180; x++ {
			if out.RGBAAt(x, y) != full.RGBAAt(x, y) {
				t.Fatalf("bad seam at %d,%d", x, y)
			}
		}
	}
}
func TestManualOverlapAndNoMovement(t *testing.T) {
	full := page(80, 500)
	first, next := tile(t, full, 0, 200), tile(t, full, 100, 200)
	s, _ := New(first)
	if _, e := s.Add(first, -1); !errors.Is(e, ErrNoMovement) {
		t.Fatalf("duplicate accepted: %v", e)
	}
	status, e := s.Add(next, 50)
	if e != nil || status.Height != 300 {
		t.Fatalf("manual overlap: %#v %v", status, e)
	}
	if _, e = s.Add(next, 120); e == nil {
		t.Fatal("invalid input accepted")
	}
}
func TestAmbiguousAndMissingOverlapAreRejected(t *testing.T) {
	a, b := image.NewRGBA(image.Rect(0, 0, 100, 160)), image.NewRGBA(image.Rect(0, 0, 100, 160))
	for y := 0; y < 160; y++ {
		for x := 0; x < 100; x++ {
			a.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			b.SetRGBA(x, y, color.RGBA{245, 245, 245, 255})
		}
	}
	if _, e := DetectOverlap(a, b); !errors.Is(e, ErrNoOverlap) {
		t.Fatalf("flat regions should need manual alignment: %v", e)
	}
	full := page(100, 700)
	if _, e := DetectOverlap(tile(t, full, 0, 180), tile(t, full, 400, 180)); !errors.Is(e, ErrNoOverlap) {
		t.Fatalf("unrelated segments accepted: %v", e)
	}
}
func TestBoundsSizeAndSegmentLimits(t *testing.T) {
	full := page(100, 1000)
	if _, e := CopyRegion(full, image.Rect(-1, 0, 60, 80)); e == nil {
		t.Fatal("out-of-screen crop accepted")
	}
	if _, e := CopyRegion(full, image.Rect(0, 0, 10, 20)); e == nil {
		t.Fatal("tiny region accepted")
	}
	s, _ := New(tile(t, full, 0, 100))
	for n := 1; n < MaxSegments; n++ {
		im := page(100, 100)
		im.SetRGBA(0, 0, color.RGBA{uint8(n), 0, 0, 255})
		if _, e := s.Add(im, 0); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.Add(tile(t, full, 200, 100), 0); e == nil {
		t.Fatal("segment limit missing")
	}
}
func TestOfferHoldsSmallMovesAndFlushes(t *testing.T) {
	full := page(160, 1400)
	s, _ := New(tile(t, full, 0, 300))
	grew, e := s.Offer(tile(t, full, 60, 300))
	if grew || e != nil {
		t.Fatalf("small move committed early: %v %v", grew, e)
	}
	// 200px past the start is more than 40% of the region: commit straight away.
	if grew, e = s.Offer(tile(t, full, 200, 300)); !grew || e != nil {
		t.Fatalf("large move not committed: %v %v", grew, e)
	}
	if grew, e = s.Offer(tile(t, full, 260, 300)); grew || e != nil {
		t.Fatalf("second small move: %v %v", grew, e)
	}
	if e = s.Flush(); e != nil {
		t.Fatal(e)
	}
	out := s.Image()
	if out.Bounds().Dy() != 560 {
		t.Fatalf("height %d", out.Bounds().Dy())
	}
	for y := 0; y < 560; y++ {
		for x := 0; x < 160; x++ {
			if out.RGBAAt(x, y) != full.RGBAAt(x, y) {
				t.Fatalf("bad pixel at %d,%d", x, y)
			}
		}
	}
}
func TestOfferBridgesWithHeldFrame(t *testing.T) {
	full := page(160, 1400)
	s, _ := New(tile(t, full, 0, 300))
	s.Offer(tile(t, full, 100, 300))
	// 330px from the committed frame has no overlap with it, but does with the held one.
	grew, e := s.Offer(tile(t, full, 330, 300))
	if !grew || e != nil {
		t.Fatalf("bridge failed: %v %v", grew, e)
	}
	s.Flush()
	out := s.Image()
	if out.Bounds().Dy() != 630 {
		t.Fatalf("height %d", out.Bounds().Dy())
	}
	for y := 0; y < 630; y += 7 {
		if out.RGBAAt(80, y) != full.RGBAAt(80, y) {
			t.Fatalf("bad pixel at row %d", y)
		}
	}
}
func TestOfferReportsRepeatedMisses(t *testing.T) {
	full := page(160, 3000)
	s, _ := New(tile(t, full, 0, 300))
	var e error
	for n := 0; n < 4; n++ {
		_, e = s.Offer(tile(t, full, 1000+n*400, 300))
	}
	if e == nil {
		t.Fatal("expected a hint after repeated misses")
	}
}
