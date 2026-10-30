package main

import (
	"errors"
	"golang.design/x/clipboard"
	"testing"
)

func TestClipboardTextFallback(t *testing.T) {
	f, b, err := readClipboardContent(func(f clipboard.Format) ([]byte, error) {
		if f == clipboard.FmtImage {
			return nil, clipboard.ErrNoData
		}
		return []byte("中文便签"), nil
	})
	if err != nil || f != clipboard.FmtText || string(b) != "中文便签" {
		t.Fatalf("text fallback failed: %v %s %v", f, b, err)
	}
}
func TestClipboardImagePreferred(t *testing.T) {
	f, b, err := readClipboardContent(func(f clipboard.Format) ([]byte, error) {
		if f != clipboard.FmtImage {
			t.Fatal("read text despite image availability")
		}
		return []byte("synthetic image"), nil
	})
	if err != nil || f != clipboard.FmtImage || len(b) == 0 {
		t.Fatal("image not selected")
	}
}
func TestClipboardUnavailableIsNotEmpty(t *testing.T) {
	_, _, err := readClipboardContent(func(clipboard.Format) ([]byte, error) { return nil, clipboard.ErrUnavailable })
	if !errors.Is(err, clipboard.ErrUnavailable) {
		t.Fatalf("lost transport error: %v", err)
	}
}
func TestEmptyClipboardHasReadableError(t *testing.T) {
	_, _, err := readClipboardContent(func(clipboard.Format) ([]byte, error) { return nil, clipboard.ErrNoData })
	if err == nil || err.Error() != "剪贴板没有图片或文字" {
		t.Fatalf("unexpected empty result: %v", err)
	}
}
