package gpuui

import "testing"

func TestFileKinds(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  bool
		want string
	}{
		{"photo.JPG", false, "image"}, {"cover.svg", false, "image"}, {"song.flac", false, "audio"}, {"clip.MOV", false, "video"},
		{"index.php", false, "php"}, {"index.phtml", false, "php"}, {"index.html", false, "html"}, {"style.scss", false, "css"}, {"script.mjs", false, "js"}, {"App.tsx", false, "ts"},
		{"package.json", false, "json"}, {"README.md", false, "markdown"}, {"archive.tar.gz", false, "archive"}, {"manual.pdf", false, "pdf"},
		{".env", false, "config"}, {".gitignore", false, "config"}, {"main.go", false, "go"}, {"odd.xyz", false, "file"}, {"LICENSE", false, "file"}, {"photos.mp3", true, "folder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fileKind(tc.name, tc.dir); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
