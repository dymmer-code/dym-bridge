package gpuui

import (
	"testing"
	"unicode/utf8"
)

func TestCompactPath(t *testing.T) {
	measure := func(s string) float32 { return float32(utf8.RuneCountInString(s)) }
	for _, tc := range []struct {
		path  string
		width float32
		want  string
	}{
		{"/Users/manuel/Projects/Dymmer/dym-bridge", 100, "/Users/manuel/Projects/Dymmer/dym-bridge"},
		{"/Users/manuel/Projects/Dymmer/dym-bridge", 30, "/Users/…/Dymmer/dym-bridge"},
		{"/Users/manuel/Projects/Dymmer/dym-bridge", 13, "/…/dym-bridge"},
		{`C:\Users\manuel\Projects\dym-bridge`, 22, `C:\Users\…\dym-bridge`},
		{"/very-long-folder-name", 8, "…er-name"},
		{"/資料/作業/写真", 8, "/資料/…/写真"},
		{"/abc", 0, ""},
		{"/abc", 1, "…"},
		{"/", 1, "/"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got := compactPath(tc.path, tc.width, measure)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if measure(got) > tc.width || !utf8.ValidString(got) {
				t.Fatal("overflow or invalid Unicode")
			}
		})
	}
}
