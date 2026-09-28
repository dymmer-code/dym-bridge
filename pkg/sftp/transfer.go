package sftp

import (
	"fmt"
	"io"
)

// ProgressCallback reports bytes transferred and total size.
type ProgressCallback func(transferred, total int64)

// CountingReader wraps an io.Reader and reports progress.
type CountingReader struct {
	reader      io.Reader
	total       int64
	transferred int64
	onProgress  ProgressCallback
}

// NewCountingReader creates a reader that tracks read progress.
func NewCountingReader(r io.Reader, total int64, cb ProgressCallback) *CountingReader {
	return &CountingReader{
		reader:     r,
		total:      total,
		onProgress: cb,
	}
}

func (cr *CountingReader) Read(p []byte) (int, error) {
	n, err := cr.reader.Read(p)
	if n > 0 {
		cr.transferred += int64(n)
		if cr.onProgress != nil {
			cr.onProgress(cr.transferred, cr.total)
		}
	}
	return n, err
}

// CountingWriter wraps an io.Writer and reports progress.
type CountingWriter struct {
	writer      io.Writer
	total       int64
	transferred int64
	onProgress  ProgressCallback
}

// NewCountingWriter creates a writer that tracks written progress.
func NewCountingWriter(w io.Writer, total int64, cb ProgressCallback) *CountingWriter {
	return &CountingWriter{
		writer:     w,
		total:      total,
		onProgress: cb,
	}
}

func (cw *CountingWriter) Write(p []byte) (int, error) {
	n, err := cw.writer.Write(p)
	if n > 0 {
		cw.transferred += int64(n)
		if cw.onProgress != nil {
			cw.onProgress(cw.transferred, cw.total)
		}
	}
	return n, err
}

// FormatFileSize returns a human-readable string representation of a byte size.
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
