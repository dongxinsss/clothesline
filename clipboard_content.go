package main

import (
	"errors"
	"golang.design/x/clipboard"
)

// Absence of the image format is normal for a text-only clipboard.
// A clipboard transport error must remain distinguishable from empty content.
func readClipboardContent(read func(clipboard.Format) ([]byte, error)) (clipboard.Format, []byte, error) {
	for _, format := range []clipboard.Format{clipboard.FmtImage, clipboard.FmtText} {
		data, err := read(format)
		if errors.Is(err, clipboard.ErrNoData) {
			continue
		}
		if err != nil {
			return format, nil, err
		}
		if len(data) > 0 {
			return format, data, nil
		}
	}
	return clipboard.FmtText, nil, errors.New("剪贴板没有图片或文字")
}
