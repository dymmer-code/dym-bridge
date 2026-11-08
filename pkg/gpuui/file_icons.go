package gpuui

import (
	"path/filepath"
	"strings"

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

// A shared 18x18 document silhouette keeps every file type, including unknown
// files, at the same optical size as the approved media pictograms.
func documentGlyph(name string, mark ...icon.PathOp) icon.IconData {
	ops := []icon.PathOp{
		icon.Move(3, 3), icon.Line(15, 3), icon.Line(21, 9), icon.Line(21, 21), icon.Line(3, 21), icon.ClosePath(),
		icon.Move(15, 3), icon.Line(15, 9), icon.Line(21, 9),
	}
	return icon.IconData{Name: name, ViewBox: 24, StrokeWidth: 1.5, Ops: append(ops, mark...)}
}

var documentIcon = documentGlyph("file")
var codeFileIcon = documentGlyph("file-code",
	icon.Move(8, 11), icon.Line(5, 14), icon.Line(8, 17),
	icon.Move(16, 11), icon.Line(19, 14), icon.Line(16, 17),
	icon.Move(13, 11), icon.Line(11, 17))
var scriptFileIcon = documentGlyph("file-script",
	icon.Move(13, 10), icon.Line(8, 15), icon.Line(12, 15), icon.Line(11, 19), icon.Line(17, 13), icon.Line(13, 13), icon.ClosePath())
var styleFileIcon = documentGlyph("file-style",
	icon.Move(16, 10), icon.Line(10, 16), icon.Line(8, 14), icon.Line(14, 8),
	icon.Move(10, 16), icon.Line(9, 19), icon.Line(5, 19), icon.Line(6, 15), icon.Line(8, 14))
var dataFileIcon = documentGlyph("file-data",
	icon.Move(9, 10), icon.Line(7, 10), icon.Line(7, 13), icon.Line(5, 14), icon.Line(7, 15), icon.Line(7, 18), icon.Line(9, 18),
	icon.Move(15, 10), icon.Line(17, 10), icon.Line(17, 13), icon.Line(19, 14), icon.Line(17, 15), icon.Line(17, 18), icon.Line(15, 18))
var configFileIcon = documentGlyph("file-config",
	icon.Move(6, 12), icon.Line(18, 12), icon.Move(6, 17), icon.Line(18, 17),
	icon.Move(9, 10), icon.Line(9, 14), icon.Move(15, 15), icon.Line(15, 19))
var textFileIcon = documentGlyph("file-text",
	icon.Move(6, 11), icon.Line(12, 11), icon.Move(6, 14), icon.Line(18, 14), icon.Move(6, 17), icon.Line(15, 17))
var markdownFileIcon = documentGlyph("file-markdown",
	icon.Move(6, 11), icon.Line(11, 11), icon.Move(6, 14), icon.Line(10, 14),
	icon.Move(15, 11), icon.Line(15, 18), icon.Move(12, 15), icon.Line(15, 18), icon.Line(18, 15))
var archiveFileIcon = documentGlyph("file-archive",
	icon.Move(10, 5), icon.Line(12, 5), icon.Line(12, 8), icon.Line(10, 8), icon.Line(10, 11), icon.Line(12, 11), icon.Line(12, 14),
	icon.Move(10, 15), icon.Line(13, 15), icon.Line(13, 18), icon.Line(10, 18), icon.ClosePath())
var pdfFileIcon = documentGlyph("file-pdf",
	icon.Move(7, 18), icon.Cubic(10, 15, 14, 6, 11, 10), icon.Cubic(9, 13, 14, 17, 18, 15), icon.Cubic(14, 14, 8, 16, 7, 18))
var databaseFileIcon = documentGlyph("file-database",
	icon.Move(7, 12), icon.Cubic(7, 10, 17, 10, 17, 12), icon.Cubic(17, 14, 7, 14, 7, 12),
	icon.Line(7, 17), icon.Cubic(7, 19, 17, 19, 17, 17), icon.Line(17, 12))

type fileIconStyle struct {
	label string
	image icon.IconData
	color uint32
}

var fileStyles = map[string]fileIconStyle{
	"php": {"PHP source", codeFileIcon, 0x7965B2}, "html": {"HTML document", codeFileIcon, 0xC45A32},
	"css": {"Stylesheet", styleFileIcon, 0x3278BC}, "js": {"JavaScript source", scriptFileIcon, 0x987511},
	"ts": {"TypeScript source", scriptFileIcon, 0x3178C6}, "go": {"Go source", codeFileIcon, 0x16849B},
	"json": {"JSON data", dataFileIcon, 0x91701F}, "config": {"Configuration file", configFileIcon, 0x687684},
	"markdown": {"Markdown document", markdownFileIcon, 0x507CA3}, "xml": {"XML document", codeFileIcon, 0xB06B32},
	"sql": {"SQL source", databaseFileIcon, 0x567FA1}, "code": {"Source code", codeFileIcon, 0x7C6BAD},
	"archive": {"Archive", archiveFileIcon, 0x9A763F}, "pdf": {"PDF document", pdfFileIcon, 0xBD4B51},
	"text": {"Text document", textFileIcon, 0x687684}, "document": {"Document", textFileIcon, 0x3E79AE},
	"image": {"Image", imageFileIcon, 0x36916B}, "audio": {"Audio", audioFileIcon, 0x9961AE}, "video": {"Video", videoFileIcon, 0xB35A7A},
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
	return icon.NewIcon(style.image, icon.Size(20), icon.Color(tint), icon.Label(style.label))
}
