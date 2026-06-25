package flatui

import "strings"

const (
	zoneMarkStart = "\x1b]9;flatui-zs="
	zoneMarkEnd   = "\x1b]9;flatui-zs-\x07"
	zoneMarkSep   = "\x07"
)

func Mark(id, content string) string {
	return zoneMarkStart + id + zoneMarkSep + content + zoneMarkEnd
}

type ZoneScanner struct {
	order []string
	rects map[string]Rect
}

func NewZoneScanner() *ZoneScanner {
	return &ZoneScanner{rects: make(map[string]Rect)}
}

func (z *ZoneScanner) Scan(frame string) {
	z.order = z.order[:0]
	clear(z.rects)

	x, y := 0, 0
	i := 0
	var openID string
	var openX, openY int

	for i < len(frame) {
		if strings.HasPrefix(frame[i:], zoneMarkStart) {
			rest := frame[i+len(zoneMarkStart):]
			end := strings.Index(rest, zoneMarkSep)
			if end < 0 {
				break
			}
			openID = rest[:end]
			openX, openY = x, y
			i += len(zoneMarkStart) + end + len(zoneMarkSep)
			continue
		}
		if strings.HasPrefix(frame[i:], zoneMarkEnd) {
			if openID != "" {
				z.record(openID, openX, openY, x, y)
				openID = ""
			}
			i += len(zoneMarkEnd)
			continue
		}

		if frame[i] == 0x1b {
			i = skipEscape(frame, i)
			continue
		}

		c := frame[i]
		if c == '\n' {
			x = 0
			y++
			i++
			continue
		}
		if c == '\r' {
			x = 0
			i++
			continue
		}
		if c == '\t' {
			x += 8 - (x % 8)
			i++
			continue
		}

		r, w := decodeRune(frame[i:])
		x += runeWidth(r)
		i += w
	}
}

func (z *ZoneScanner) record(id string, x1, y1, x2, y2 int) {
	rect := Rect{X: x1, Y: y1}
	if y1 == y2 {
		rect.Width = x2 - x1
		rect.Height = 1
	} else {
		rect.Width = x2 - x1
		rect.Height = y2 - y1 + 1
	}
	if rect.Width < 0 {
		rect.Width = 0
	}
	if _, exists := z.rects[id]; !exists {
		z.order = append(z.order, id)
	} else {
		prev := z.rects[id]
		minX := min(prev.X, rect.X)
		minY := min(prev.Y, rect.Y)
		maxX := max(prev.X+prev.Width, rect.X+rect.Width)
		maxY := max(prev.Y+prev.Height, rect.Y+rect.Height)
		rect = Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
	}
	z.rects[id] = rect
}

func (z ZoneScanner) At(x, y int) (string, bool) {
	for i := len(z.order) - 1; i >= 0; i-- {
		id := z.order[i]
		if z.rects[id].Contains(x, y) {
			return id, true
		}
	}
	return "", false
}

func (z ZoneScanner) Rect(id string) (Rect, bool) {
	r, ok := z.rects[id]
	return r, ok
}

func (z ZoneScanner) In(id string, x, y int) bool {
	r, ok := z.rects[id]
	return ok && r.Contains(x, y)
}

func skipEscape(s string, i int) int {
	if i+1 >= len(s) {
		return i + 1
	}
	switch s[i+1] {
	case '[':
		j := i + 2
		for j < len(s) {
			c := s[j]
			if c >= 0x40 && c <= 0x7e {
				return j + 1
			}
			j++
		}
		return j
	case ']':
		j := i + 2
		for j < len(s) {
			if s[j] == 0x07 {
				return j + 1
			}
			if s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\' {
				return j + 2
			}
			j++
		}
		return j
	case 'P', 'X', '^', '_':
		j := i + 2
		for j+1 < len(s) {
			if s[j] == 0x1b && s[j+1] == '\\' {
				return j + 2
			}
			j++
		}
		return len(s)
	default:
		return i + 2
	}
}

func decodeRune(s string) (rune, int) {
	if len(s) == 0 {
		return 0, 0
	}
	b := s[0]
	if b < 0x80 {
		return rune(b), 1
	}
	r, w := decodeMultiByte(s)
	return r, w
}

func decodeMultiByte(s string) (rune, int) {
	b := s[0]
	switch {
	case b&0xe0 == 0xc0 && len(s) >= 2:
		return rune(b&0x1f)<<6 | rune(s[1]&0x3f), 2
	case b&0xf0 == 0xe0 && len(s) >= 3:
		return rune(b&0x0f)<<12 | rune(s[1]&0x3f)<<6 | rune(s[2]&0x3f), 3
	case b&0xf8 == 0xf0 && len(s) >= 4:
		return rune(b&0x07)<<18 | rune(s[1]&0x3f)<<12 | rune(s[2]&0x3f)<<6 | rune(s[3]&0x3f), 4
	}
	return 0xfffd, 1
}

func runeWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 0x20 || r == 0x7f:
		return 0
	case r >= 0x1100 && r <= 0x115f,
		r >= 0x2e80 && r <= 0x303e,
		r >= 0x3041 && r <= 0x33ff,
		r >= 0x3400 && r <= 0x4dbf,
		r >= 0x4e00 && r <= 0x9fff,
		r >= 0xa000 && r <= 0xa4cf,
		r >= 0xac00 && r <= 0xd7a3,
		r >= 0xf900 && r <= 0xfaff,
		r >= 0xfe30 && r <= 0xfe4f,
		r >= 0xff00 && r <= 0xff60,
		r >= 0xffe0 && r <= 0xffe6,
		r >= 0x20000 && r <= 0x2fffd,
		r >= 0x30000 && r <= 0x3fffd:
		return 2
	default:
		return 1
	}
}
