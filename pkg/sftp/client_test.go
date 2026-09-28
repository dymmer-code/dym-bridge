package sftp

import (
	"bytes"
	"io"
	"testing"
)

func TestSortItems(t *testing.T) {
	items := []Item{
		{Name: "zebra.txt", IsDir: false},
		{Name: "uploads", IsDir: true},
		{Name: "apple.txt", IsDir: false},
		{Name: "themes", IsDir: true},
		{Name: "Beta.txt", IsDir: false},
	}

	SortItems(items)

	// Directories first, alphabetically: themes, uploads
	if !items[0].IsDir || items[0].Name != "themes" {
		t.Errorf("expected items[0] to be directory 'themes', got %+v", items[0])
	}
	if !items[1].IsDir || items[1].Name != "uploads" {
		t.Errorf("expected items[1] to be directory 'uploads', got %+v", items[1])
	}

	// Files next, case-insensitive: apple.txt, Beta.txt, zebra.txt
	if items[2].IsDir || items[2].Name != "apple.txt" {
		t.Errorf("expected items[2] to be 'apple.txt', got %+v", items[2])
	}
	if items[3].IsDir || items[3].Name != "Beta.txt" {
		t.Errorf("expected items[3] to be 'Beta.txt', got %+v", items[3])
	}
	if items[4].IsDir || items[4].Name != "zebra.txt" {
		t.Errorf("expected items[4] to be 'zebra.txt', got %+v", items[4])
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{52428800, "50.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		got := FormatFileSize(tt.bytes)
		if got != tt.expected {
			t.Errorf("FormatFileSize(%d) = %q, expected %q", tt.bytes, got, tt.expected)
		}
	}
}

func TestCountingReader(t *testing.T) {
	data := []byte("Hello, Dymmer Bridge!")
	var recordedTransferred int64
	var recordedTotal int64

	cb := func(transferred, total int64) {
		recordedTransferred = transferred
		recordedTotal = total
	}

	rawReader := bytes.NewReader(data)
	counting := NewCountingReader(rawReader, int64(len(data)), cb)

	out, err := io.ReadAll(counting)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}

	if string(out) != string(data) {
		t.Errorf("read content mismatch: %q", string(out))
	}
	if recordedTransferred != int64(len(data)) {
		t.Errorf("expected transferred %d, got %d", len(data), recordedTransferred)
	}
	if recordedTotal != int64(len(data)) {
		t.Errorf("expected total %d, got %d", len(data), recordedTotal)
	}
}
