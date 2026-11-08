package gpuui

import "strings"

// compactPath only changes presentation. Measure uses the actual canvas font,
// so the result adapts to resizing and does not assume fixed-width characters.
func compactPath(path string, width float32, measure func(string) float32) string {
	if measure(path) <= width {
		return path
	}
	if width <= 0 || measure("…") > width {
		return ""
	}
	sep := "/"
	if strings.Contains(path, `\`) && !strings.Contains(path, "/") {
		sep = `\`
	}
	parts := strings.Split(path, sep)
	head := 1
	if strings.HasPrefix(path, sep) {
		head = 2
	}
	if strings.HasPrefix(path, sep+sep) {
		head = 4
	} // UNC server/share
	tail := len(parts) - 1
	candidate := func(h, t int) string {
		prefix := ""
		if h > 0 {
			prefix = strings.Join(parts[:h], sep) + sep
		}
		return prefix + "…" + sep + strings.Join(parts[t:], sep)
	}
	for head >= 0 {
		if head < tail && measure(candidate(head, tail)) <= width {
			// Prefer complete trailing folders over extra leading components.
			for tail > head+1 && measure(candidate(head, tail-1)) <= width {
				tail--
			}
			for head+1 < tail && measure(candidate(head+1, tail)) <= width {
				head++
			}
			return candidate(head, tail)
		}
		head--
	}
	// Even the last folder does not fit: retain its ending without splitting UTF-8.
	runes := []rune(path)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if measure("…"+string(runes[len(runes)-mid:])) <= width {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return "…" + string(runes[len(runes)-lo:])
}

func (t *cellText) Path() *cellText { t.path = true; return t }
