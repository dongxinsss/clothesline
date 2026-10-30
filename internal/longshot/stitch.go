// Package longshot joins captures of one fixed rectangle after manual scrolling.
package longshot

import (
	"errors"
	"image"
	"image/draw"
	"math"
)

// Segments are stored without their overlap, so memory follows the output
// size; the pixel and height caps are the real limits.
const MaxSegments = 60
const MaxPixels = 24_000_000

var ErrNoMovement = errors.New("画面没有变化，请先向下滚动页面再采集")
var ErrNoOverlap = errors.New("未找到可靠重叠。请向下滚动约半屏后重试，或选择手动重叠比例")

// Offer keeps a frame until the page has moved this share of the region, so
// slow scrolling does not produce many thin segments.
const commitShare = 0.4

type Status struct {
	Count   int    `json:"count"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Overlap int    `json:"overlap"`
	Auto    bool   `json:"auto"`
	Hint    string `json:"hint,omitempty"`
}
type Session struct {
	parts                      []*image.RGBA // committed segments without their overlap
	last                       *image.RGBA   // last committed full frame, for matching
	pending                    *image.RGBA   // matched frame that has not moved far enough yet
	pendingOverlap, misses     int
	width, height, lastOverlap int
}

func CopyRegion(src image.Image, rect image.Rectangle) (*image.RGBA, error) {
	if rect.Dx() < 40 || rect.Dy() < 40 || !rect.In(src.Bounds()) {
		return nil, errors.New("请选择至少 40 × 40 像素、位于屏幕内的区域")
	}
	dst := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(dst, dst.Bounds(), src, rect.Min, draw.Src)
	return dst, nil
}
func New(first *image.RGBA) (*Session, error) {
	if first == nil || first.Bounds().Dx() < 40 || first.Bounds().Dy() < 40 {
		return nil, errors.New("截取区域太小")
	}
	w, h := first.Bounds().Dx(), first.Bounds().Dy()
	if int64(w)*int64(h) > MaxPixels {
		return nil, errors.New("截取区域过大")
	}
	return &Session{parts: []*image.RGBA{first}, last: first, width: w, height: h}, nil
}
func (s *Session) Status() Status {
	return Status{Count: len(s.parts), Width: s.width, Height: s.height, Overlap: s.lastOverlap}
}
func same(a, b *image.RGBA) bool {
	if a.Bounds().Size() != b.Bounds().Size() {
		return false
	}
	// Sparse pass avoids a full scan for clearly different frames.
	for y := 0; y < a.Bounds().Dy(); y += 17 {
		for x := 0; x < a.Bounds().Dx(); x += 19 {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				return false
			}
		}
	}
	for y := 0; y < a.Bounds().Dy(); y++ {
		for x := 0; x < a.Bounds().Dx(); x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				return false
			}
		}
	}
	return true
}

// DetectOverlap compares pixels in the previous bottom and next top. Flat or
// ambiguous regions are rejected so an uncertain match never silently cuts text.
func DetectOverlap(previous, next *image.RGBA) (int, error) {
	if previous.Bounds().Size() != next.Bounds().Size() {
		return 0, errors.New("两段尺寸不同，请重新选择区域")
	}
	if same(previous, next) {
		return 0, ErrNoMovement
	}
	w, h := previous.Bounds().Dx(), previous.Bounds().Dy()
	minOverlap := max(24, h/12)
	maxOverlap := h - max(16, h/25)
	best, bestError, bestTexture := 0, math.Inf(1), 0.0
	errorAt := func(overlap int) (float64, float64) {
		var sum, texture float64
		count := 0
		for iy := 1; iy <= 28; iy++ {
			y := iy * (overlap - 1) / 29
			for ix := 1; ix <= 36; ix++ {
				x := ix * (w - 1) / 37
				a := previous.RGBAAt(x, h-overlap+y)
				b := next.RGBAAt(x, y)
				sum += math.Abs(float64(a.R)-float64(b.R)) + math.Abs(float64(a.G)-float64(b.G)) + math.Abs(float64(a.B)-float64(b.B))
				count += 3
				neighbor := next.RGBAAt(min(w-1, x+3), y)
				texture += math.Abs(float64(b.R)-float64(neighbor.R)) + math.Abs(float64(b.G)-float64(neighbor.G)) + math.Abs(float64(b.B)-float64(neighbor.B))
			}
		}
		return sum / float64(count), texture / float64(count)
	}
	// Exhaustive offsets keep joins exact at Retina resolution too.
	for overlap := minOverlap; overlap <= maxOverlap; overlap++ {
		score, texture := errorAt(overlap)
		if texture < 1.2 {
			continue
		}
		if score < bestError {
			best, bestError, bestTexture = overlap, score, texture
		}
	}
	if best == 0 || bestError > min(7.0, bestTexture*.45) {
		return 0, ErrNoOverlap
	}
	// Repeated patterns can match multiple offsets; request manual alignment.
	for overlap := minOverlap; overlap <= maxOverlap; overlap++ {
		if abs(overlap-best) < max(4, h/100) {
			continue
		}
		score, texture := errorAt(overlap)
		if texture >= 1.2 && score <= bestError+.6 {
			return 0, ErrNoOverlap
		}
	}
	return best, nil
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func (s *Session) Add(next *image.RGBA, overlapPercent int) (Status, error) {
	if next == nil || next.Bounds().Dx() != s.width || next.Bounds().Dy() != s.last.Bounds().Dy() {
		return s.Status(), errors.New("屏幕或区域尺寸变化，请重新开始")
	}
	if same(s.last, next) {
		return s.Status(), ErrNoMovement
	}
	overlap := 0
	if overlapPercent == -1 {
		var err error
		overlap, err = DetectOverlap(s.last, next)
		if err != nil {
			return s.Status(), err
		}
	} else if overlapPercent >= 0 && overlapPercent <= 85 {
		overlap = next.Bounds().Dy() * overlapPercent / 100
	} else {
		return s.Status(), errors.New("重叠比例必须为自动或 0% 到 85%")
	}
	if err := s.commit(next, overlap); err != nil {
		return s.Status(), err
	}
	return s.Status(), nil
}
func (s *Session) commit(next *image.RGBA, overlap int) error {
	if len(s.parts) >= MaxSegments {
		return errors.New("已达到分段上限，请完成当前长图")
	}
	h := next.Bounds().Dy() - overlap
	height := s.height + h
	if int64(height)*int64(s.width) > MaxPixels || height > 32760 {
		return errors.New("长图已达到大小上限，请先完成当前长图")
	}
	part := image.NewRGBA(image.Rect(0, 0, s.width, h))
	draw.Draw(part, part.Bounds(), next, image.Pt(0, overlap), draw.Src)
	s.parts = append(s.parts, part)
	s.last, s.pending = next, nil
	s.height, s.lastOverlap, s.misses = height, overlap, 0
	return nil
}

// Offer takes a frame captured while the user scrolls and reports whether the
// long image grew. Frames that moved only a little are held back and committed
// once the page moves further, or when the next frame no longer matches.
func (s *Session) Offer(next *image.RGBA) (bool, error) {
	if next == nil || next.Bounds().Dx() != s.width || next.Bounds().Dy() != s.last.Bounds().Dy() {
		return false, errors.New("屏幕或区域尺寸变化，请重新开始")
	}
	if same(s.last, next) || (s.pending != nil && same(s.pending, next)) {
		return false, nil
	}
	overlap, err := DetectOverlap(s.last, next)
	if err != nil && s.pending != nil {
		// Moved past the last committed frame: keep the held one as a bridge.
		if err = s.commit(s.pending, s.pendingOverlap); err != nil {
			return false, err
		}
		overlap, err = DetectOverlap(s.last, next)
		if err != nil {
			return true, nil
		}
	}
	if err != nil {
		s.misses++
		if s.misses >= 4 {
			return false, errors.New("没有对上上一段，请稍微往回滚动一点，或放慢滚动速度")
		}
		return false, nil
	}
	s.misses = 0
	if float64(overlap) <= float64(next.Bounds().Dy())*(1-commitShare) {
		return true, s.commit(next, overlap)
	}
	s.pending, s.pendingOverlap = next, overlap
	return false, nil
}

// Flush commits a held frame so the last bit of scrolling is not lost.
func (s *Session) Flush() error {
	if s.pending == nil {
		return nil
	}
	return s.commit(s.pending, s.pendingOverlap)
}
func (s *Session) Image() *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, s.width, s.height))
	y := 0
	for _, part := range s.parts {
		h := part.Bounds().Dy()
		draw.Draw(out, image.Rect(0, y, s.width, y+h), part, image.Point{}, draw.Src)
		y += h
	}
	return out
}
