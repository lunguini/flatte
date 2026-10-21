// Command flat-gallery shows the layout vocabulary that is richer in a
// window: text at point sizes, rich text that wraps and (in a window) can be
// selected and copied, pictures, and icons. The terminal shows each in its
// own spelling — the pictures as their descriptions — and, with -gui, a
// native window draws them.
package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/window"
)

// picture is one picture in the gallery.
type picture struct {
	name, text string
	img        image.Image
}

type State struct {
	pictures []picture
	selected int
	fit      layout.ImageFit
}

func NewState() *State {
	return &State{pictures: []picture{
		{name: "sunset", text: "a sun setting over hills", img: sunset(240, 160)},
		{name: "tiles", text: "a checkerboard of tiles", img: tiles(160, 160)},
		{name: "waves", text: "bands of waves", img: waves(320, 120)},
	}}
}

var fits = []string{"Contain", "Cover", "Stretch"}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.KeyEvent:
		switch e.Key {
		case flatte.KeyEscape, flatte.KeyCtrlC:
			fx.Quit()
		case flatte.KeyLeft:
			s.selected = (s.selected + len(s.pictures) - 1) % len(s.pictures)
		case flatte.KeyRight, flatte.KeyTab:
			s.selected = (s.selected + 1) % len(s.pictures)
		case flatte.KeyCharacter:
			switch e.Rune {
			case 'q':
				fx.Quit()
			case 'f':
				s.fit = (s.fit + 1) % layout.ImageFit(len(fits))
			}
		}
	case flatte.MouseEvent:
		if e.Button != flatte.MouseLeft || e.Action != flatte.MousePress {
			return
		}
		for i, p := range s.pictures {
			if e.Target == p.name {
				s.selected = i
			}
		}
	}
}

var accent = lipgloss.Color("#5fafd7")

// icons are the icon set, with their terminal spellings.
var icons = []struct {
	name  layout.IconName
	text  string
	label string
}{
	{layout.IconCheck, "✓ ", "check"}, {layout.IconClose, "✕ ", "close"},
	{layout.IconPlus, "+ ", "plus"}, {layout.IconMinus, "− ", "minus"},
	{layout.IconSearch, "⌕ ", "search"}, {layout.IconGear, "⚙ ", "gear"},
	{layout.IconInfo, "ⓘ ", "info"}, {layout.IconWarning, "⚠ ", "warning"},
	{layout.IconError, "⊗ ", "error"}, {layout.IconEllipsis, "… ", "more"},
	{layout.IconArrowUp, "↑ ", "up"}, {layout.IconArrowDown, "↓ ", "down"},
	{layout.IconArrowLeft, "← ", "left"}, {layout.IconArrowRight, "→ ", "right"},
	{layout.IconChevronLeft, "‹ ", "back"}, {layout.IconChevronRight, "› ", "next"},
	{layout.IconChevronUp, "˄ ", "less"}, {layout.IconChevronDown, "˅ ", "expand"},
	{layout.IconFolder, "▭ ", "folder"}, {layout.IconFile, "▯ ", "file"},
}

// iconsPerRow is how many icons a row of the icon grid holds.
const iconsPerRow = 5

func heading(text string) layout.Node {
	return layout.Label{NodeBase: layout.NodeBase{PadTop: 1}, Text: text, Style: layout.TextStyle{Bold: true, Size: 15}}
}

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	b := layout.TextStyle{Bold: true}
	prose := layout.Label{Wrap: true, Selectable: true, Runs: []layout.Run{
		{Text: "Labels hold "},
		{Text: "rich text", Style: b},
		{Text: " in runs: "},
		{Text: "italic", Style: layout.TextStyle{Italic: true}},
		{Text: ", "},
		{Text: "colored", Style: layout.TextStyle{Fg: accent}},
		{Text: ", "},
		{Text: "underlined", Style: layout.TextStyle{Underline: true}},
		{Text: ", or "},
		{Text: "larger", Style: layout.TextStyle{Size: 20}},
		{Text: " — and the runs wrap together as one paragraph. In a window, drag across this text and copy it with ⌘C."},
	}}

	gallery := layout.Row{NodeBase: layout.NodeBase{Gap: 2}}
	for i, p := range s.pictures {
		box := layout.Box{Border: layout.BorderRounded}
		if i == s.selected {
			box.BorderColor = accent
		}
		gallery.Children = append(gallery.Children, layout.Col{Children: []layout.Node{
			layout.Image{
				NodeBase: layout.NodeBase{ID: p.name, W: layout.Points(160), H: layout.Points(96), Box: box, Tooltip: p.text},
				Source:   p.img, Text: p.text, Fit: s.fit,
			},
			layout.Label{Text: fmt.Sprintf("%s  %d×%d", p.name, p.img.Bounds().Dx(), p.img.Bounds().Dy()), Style: layout.TextStyle{Role: layout.TextSubtle}},
		}})
	}

	grid := layout.Col{}
	for i := 0; i < len(icons); i += iconsPerRow {
		row := layout.Row{NodeBase: layout.NodeBase{Gap: 1}}
		for _, ic := range icons[i:min(i+iconsPerRow, len(icons))] {
			row.Children = append(row.Children, layout.Row{NodeBase: layout.NodeBase{W: layout.Fixed(12)}, Children: []layout.Node{
				layout.Icon{Name: ic.name, Text: ic.text, Style: layout.TextStyle{Fg: accent}},
				layout.Label{Text: ic.label},
			}})
		}
		grid.Children = append(grid.Children, row)
	}

	return flatte.Frame{Title: "Gallery", Root: layout.Col{
		NodeBase: layout.NodeBase{Pad: 1},
		Children: []layout.Node{
			layout.Label{Text: "Gallery", Style: layout.TextStyle{Role: layout.TextTitle, Size: 26}},
			heading("Text"),
			prose,
			heading("Pictures"),
			gallery,
			heading("Icons"),
			grid,
			layout.NewSpacer(),
			layout.Label{
				Text:  fmt.Sprintf("fit: %s   f change fit  ←/→ choose  q quit", fits[s.fit]),
				Style: layout.TextStyle{Role: layout.TextSubtle},
			},
		},
	}}
}

// The pictures are drawn here rather than loaded, so the sample has no
// files to find.

func sunset(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	sx, sy, sr := float64(w)*0.62, float64(h)*0.58, float64(h)*0.2
	for y := range h {
		t := float64(y) / float64(h)
		sky := mix(color.RGBA{58, 28, 92, 255}, color.RGBA{255, 140, 66, 255}, t*1.3)
		for x := range w {
			c := sky
			if math.Hypot(float64(x)-sx, float64(y)-sy) < sr {
				c = color.RGBA{255, 214, 102, 255}
			}
			hill := float64(h)*0.72 + 10*math.Sin(float64(x)/23) + 6*math.Sin(float64(x)/9)
			if float64(y) > hill {
				c = mix(color.RGBA{40, 64, 52, 255}, color.RGBA{18, 30, 26, 255}, (float64(y)-hill)/float64(h)*3)
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func tiles(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{236, 232, 220, 255}
			if (x/20+y/20)%2 == 0 {
				c = color.RGBA{95, 175, 215, 255}
			}
			if x%20 == 0 || y%20 == 0 {
				c = color.RGBA{60, 60, 70, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func waves(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := (math.Sin(float64(x)/18+float64(y)/30*math.Pi) + 1) / 2
			band := int(float64(y)/float64(h)*4+v) % 2
			c := mix(color.RGBA{20, 70, 120, 255}, color.RGBA{120, 200, 220, 255}, v)
			if band == 1 {
				c = mix(c, color.RGBA{255, 255, 255, 255}, 0.25)
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	t = min(max(t, 0), 1)
	l := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}

func main() {
	gui := flag.Bool("gui", false, "run in a native window instead of the terminal")
	flag.Parse()
	app := flatte.App[State]{State: NewState(), Handle: Handle, View: View}
	var err error
	if *gui {
		err = window.Run(context.Background(), app, window.Options{Title: "Gallery", Width: 80, Height: 34})
	} else {
		err = flatte.Run(context.Background(), app, flatte.WithMouse(flatte.MouseModeCellMotion), flatte.WithoutDefaultQuit())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
