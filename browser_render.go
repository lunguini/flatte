package flatte

import (
	"fmt"
	"image/color"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
)

// browserSpan is a DOM-neutral run of cells. The js/wasm host creates text
// nodes from Text, so application content is never interpreted as HTML.
type browserSpan struct {
	Text                                   string
	Foreground, Background, UnderlineColor string
	Link                                   string
	Underline                              string
	Cells                                  int
	Bold, Faint, Italic, Blink             bool
	Reverse, Conceal, Strikethrough        bool
}

func browserSpans(content string) [][]browserSpan {
	styled := uv.NewStyledString(content)
	area := styled.Bounds()
	buf := uv.NewScreenBuffer(area.Dx(), area.Dy())
	styled.Draw(buf, area)
	lines := buf.Lines
	out := make([][]browserSpan, len(lines))
	for y, line := range lines {
		end := len(line)
		for end > 0 && line[end-1].Equal(&uv.EmptyCell) {
			end--
		}
		for i := range line[:end] {
			cell := &line[i]
			if cell.IsZero() { // continuation cell of a wide grapheme
				continue
			}
			span := browserSpan{
				Text:           cell.Content,
				Foreground:     browserColor(cell.Style.Fg),
				Background:     browserColor(cell.Style.Bg),
				UnderlineColor: browserColor(cell.Style.UnderlineColor),
				Link:           safeBrowserLink(cell.Link.URL),
				Underline:      browserUnderline(cell.Style.Underline),
				Cells:          max(cell.Width, 1),
				Bold:           cell.Style.Attrs&uv.AttrBold != 0,
				Faint:          cell.Style.Attrs&uv.AttrFaint != 0,
				Italic:         cell.Style.Attrs&uv.AttrItalic != 0,
				Blink:          cell.Style.Attrs&(uv.AttrBlink|uv.AttrRapidBlink) != 0,
				Reverse:        cell.Style.Attrs&uv.AttrReverse != 0,
				Conceal:        cell.Style.Attrs&uv.AttrConceal != 0,
				Strikethrough:  cell.Style.Attrs&uv.AttrStrikethrough != 0,
			}
			row := out[y]
			if len(row) > 0 && span.Cells == 1 && row[len(row)-1].Cells == len([]rune(row[len(row)-1].Text)) && sameBrowserStyle(row[len(row)-1], span) {
				row[len(row)-1].Text += span.Text
				row[len(row)-1].Cells++
				out[y] = row
				continue
			}
			out[y] = append(row, span)
		}
	}
	return out
}

func sameBrowserStyle(a, b browserSpan) bool {
	a.Text, b.Text, a.Cells, b.Cells = "", "", 0, 0
	return a == b
}

func browserColor(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

func browserUnderline(style uv.Underline) string {
	switch style {
	case uv.UnderlineSingle:
		return "solid"
	case uv.UnderlineDouble:
		return "double"
	case uv.UnderlineCurly:
		return "wavy"
	case uv.UnderlineDotted:
		return "dotted"
	case uv.UnderlineDashed:
		return "dashed"
	default:
		return ""
	}
}

func safeBrowserLink(raw string) string {
	if raw == "" || strings.TrimSpace(raw) != raw {
		return ""
	}
	colon := strings.IndexByte(raw, ':')
	if colon < 0 {
		return raw
	}
	if colon == 0 || !isBrowserSchemeStart(raw[0]) {
		return ""
	}
	for i := 1; i < colon; i++ {
		if !isBrowserSchemePart(raw[i]) {
			return ""
		}
	}
	switch strings.ToLower(raw[:colon]) {
	case "http", "https", "mailto":
		return raw
	default:
		return ""
	}
}

func isBrowserSchemeStart(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func isBrowserSchemePart(b byte) bool {
	return isBrowserSchemeStart(b) || b >= '0' && b <= '9' || b == '+' || b == '-' || b == '.'
}
