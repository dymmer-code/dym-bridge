package gpuui

import (
	"path/filepath"
	"strings"

	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/icon"
	"github.com/gogpu/ui/widget"
)

// Classify names only: no content reads, thumbnails or per-file image caches.
var fileExtensions = map[string]string{}

func init() {
	for kind, extensions := range map[string]string{
		"image": "jpg jpeg png gif webp avif heic heif svg bmp tif tiff ico psd",
		"audio": "mp3 wav flac aac m4a ogg opus aiff mid midi",
		"video": "mp4 mov mkv avi webm m4v mpg mpeg wmv",
		"php":   "php php3 php4 php5 php7 php8 phtml",
		"html":  "html htm xhtml", "css": "css scss sass less",
		"js": "js jsx mjs cjs", "ts": "ts tsx mts cts", "go": "go",
		"json": "json jsonc jsonl", "config": "yaml yml toml ini conf cfg env",
		"markdown": "md markdown mdx", "xml": "xml xsl xslt", "sql": "sql",
		"code":    "py rb rs c h cpp hpp java kt swift ex exs sh bash zsh",
		"archive": "zip gz bz2 xz tar tgz 7z rar zst", "pdf": "pdf",
		"text": "txt log csv", "document": "doc docx odt rtf",
	} {
		for _, ext := range strings.Fields(extensions) {
			fileExtensions[ext] = kind
		}
	}
}
func fileKind(name string, directory bool) string {
	if directory {
		return "folder"
	}
	lower := strings.ToLower(name)
	if lower == ".env" || strings.HasPrefix(lower, ".env.") || lower == ".gitignore" || lower == ".editorconfig" || lower == "dockerfile" {
		return "config"
	}
	if kind, ok := fileExtensions[strings.TrimPrefix(filepath.Ext(lower), ".")]; ok {
		return kind
	}
	return "file"
}

var imageFileIcon = icon.IconData{Name: "image-file", ViewBox: 24, Ops: []icon.PathOp{
	icon.Move(3, 4), icon.Line(21, 4), icon.Line(21, 20), icon.Line(3, 20), icon.ClosePath(),
	icon.Move(3, 17), icon.Line(9, 11), icon.Line(14, 16), icon.Line(17, 13), icon.Line(21, 17),
	icon.Move(16, 8), icon.Line(18, 8), icon.Line(18, 10), icon.Line(16, 10), icon.ClosePath(),
}}
var audioFileIcon = icon.IconData{Name: "audio-file", ViewBox: 24, StrokeWidth: 2, Ops: []icon.PathOp{
	icon.Move(9, 18), icon.Line(9, 5), icon.Line(20, 3), icon.Line(20, 16),
	icon.Move(9, 9), icon.Line(20, 7),
	icon.Move(9, 18), icon.Line(6, 17), icon.Line(3, 19), icon.Line(4, 21), icon.Line(7, 21), icon.ClosePath(),
	icon.Move(20, 16), icon.Line(17, 15), icon.Line(14, 17), icon.Line(15, 19), icon.Line(18, 19), icon.ClosePath(),
}}
var videoFileIcon = icon.IconData{Name: "video-file", ViewBox: 24, Ops: []icon.PathOp{
	icon.Move(3, 4), icon.Line(21, 4), icon.Line(21, 20), icon.Line(3, 20), icon.ClosePath(),
	icon.Move(9, 8), icon.Line(16, 12), icon.Line(9, 16), icon.ClosePath(),
}}

type fileIconStyle struct {
	label, badge string
	color        uint32
}

var fileStyles = map[string]fileIconStyle{
	"php": {"PHP source", "PHP", 0x7965B2}, "html": {"HTML document", "<>", 0xC45A32},
	"css": {"Stylesheet", "CSS", 0x3278BC}, "js": {"JavaScript source", "JS", 0x987511},
	"ts": {"TypeScript source", "TS", 0x3178C6}, "go": {"Go source", "Go", 0x16849B},
	"json": {"JSON data", "{}", 0x91701F}, "config": {"Configuration file", "CFG", 0x687684},
	"markdown": {"Markdown document", "MD", 0x507CA3}, "xml": {"XML document", "XML", 0xB06B32},
	"sql": {"SQL source", "SQL", 0x567FA1}, "code": {"Source code", "</>", 0x7C6BAD},
	"archive": {"Archive", "ZIP", 0x9A763F}, "pdf": {"PDF document", "PDF", 0xBD4B51},
	"text": {"Text document", "TXT", 0x687684}, "document": {"Document", "DOC", 0x3E79AE},
	"image": {"Image", "", 0x36916B}, "audio": {"Audio", "", 0x9961AE}, "video": {"Video", "", 0xB35A7A},
}

type fileBadge struct {
	*icon.IconWidget
	text string
	tint widget.Color
}

func (g *fileBadge) Draw(_ widget.Context, c widget.Canvas) {
	bounds := g.Bounds()
	box := geometry.NewRect(bounds.Min.X, bounds.Min.Y+3, 20, 14)
	c.DrawRoundRect(box, g.tint.WithAlpha(.14), 2)
	c.StrokeRoundRect(box, g.tint.WithAlpha(.7), 2, .7)
	c.DrawText(g.text, box, 7.5, g.tint, true, widget.TextAlignCenter)
}
func (b *browser) fileGlyph(name string, directory bool) widget.Widget {
	kind := fileKind(name, directory)
	cs := b.theme.Colors
	if kind == "folder" {
		return icon.NewIcon(icon.FolderClosed, icon.Size(20), icon.Color(cs.Accent), icon.Label("Folder"))
	}
	style, ok := fileStyles[kind]
	if !ok {
		return icon.NewIcon(documentIcon, icon.Size(20), icon.Color(cs.OnSurfaceSecond), icon.Label("File"))
	}
	tint := widget.Hex(style.color)
	if b.dark {
		tint = tint.Lerp(widget.Hex(0xFFFFFF), .3)
	}
	image := documentIcon
	switch kind {
	case "image":
		image = imageFileIcon
	case "audio":
		image = audioFileIcon
	case "video":
		image = videoFileIcon
	}
	base := icon.NewIcon(image, icon.Size(20), icon.Color(tint), icon.Label(style.label))
	if style.badge != "" {
		return &fileBadge{IconWidget: base, text: style.badge, tint: tint}
	}
	return base
}
