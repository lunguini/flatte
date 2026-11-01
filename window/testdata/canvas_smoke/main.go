//go:build js && wasm

package main

import (
	"image"
	"image/color"
	"syscall/js"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

type state struct{ art image.Image }

func smokeApp() flatte.App[state] {
	smokeState := &state{art: image.NewRGBA(image.Rect(0, 0, 24, 24))}
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			smokeState.art.(*image.RGBA).Set(x, y, color.NRGBA{R: uint8(40 + x*7), G: uint8(80 + y*5), B: 220, A: 255})
		}
	}
	return flatte.App[state]{
		State: smokeState,
		Handle: func(_ *state, event flatte.Event, _ flatte.Effects[state]) {
			dom := js.Global().Get("__flatteTest")
			switch ev := event.(type) {
			case flatte.LayoutEvent:
				if r, ok := ev.PointRects["smoke-input"]; ok {
					dom.Set("canvasInputRect", map[string]any{"x": r.X, "y": r.Y, "w": r.W, "h": r.H})
				}
			case flatte.KeyEvent:
				if ev.Key == flatte.KeyCharacter && ev.Rune == 'x' {
					dom.Set("canvasKeySeen", true)
				}
			case flatte.MouseEvent:
				dom.Set("canvasMouseSeen", true)
				dom.Set("canvasMouseTarget", ev.Target)
				if ev.HasCaret {
					dom.Set("canvasCaretSeen", true)
				}
			}
		},
		View: func(s *state, ctx flatte.RenderContext) flatte.Frame {
			dom := js.Global().Get("__flatteTest")
			dom.Set("canvasSmokeReady", true)
			dom.Set("canvasGraphical", ctx.Graphical)
			root := layout.Col{NodeBase: layout.NodeBase{Gap: 1, Box: layout.Box{Background: color.RGBA{R: 16, G: 18, B: 24, A: 255}}}, Children: []layout.Node{
				layout.Label{NodeBase: layout.NodeBase{ID: "heading", H: layout.Points(24), Box: layout.Box{Border: layout.BorderNormal, Title: "Window scene"}}, Text: "Canvas text and wrapping", Style: layout.TextStyle{Role: layout.TextTitle}, Wrap: true, Runs: []layout.Run{{Text: "Canvas ", Style: layout.TextStyle{Bold: true}}, {Text: "text and wrapping", Style: layout.TextStyle{Role: layout.TextSubtle}}}},
				layout.Input{NodeBase: layout.NodeBase{ID: "smoke-input", H: layout.Points(36)}, Value: "pointing caret", Caret: 7, Focused: true, Clearable: true},
				layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Children: []layout.Node{
					layout.Button{NodeBase: layout.NodeBase{ID: "smoke-button", W: layout.Points(120), H: layout.Points(40), Tooltip: "canvas button"}, Label: "Continue", Default: true, Focused: true},
					layout.Button{NodeBase: layout.NodeBase{ID: "icon-button", W: layout.Points(40), H: layout.Points(40)}, Icon: layout.IconSearch, Label: "Search"},
					layout.Toggle{NodeBase: layout.NodeBase{ID: "switch", W: layout.Points(120), H: layout.Points(40)}, Kind: layout.ToggleSwitch, Label: "Enabled", Checked: true},
					layout.Toggle{NodeBase: layout.NodeBase{ID: "radio", W: layout.Points(110), H: layout.Points(40)}, Kind: layout.ToggleRadio, Label: "Selected", Checked: true},
				}},
				layout.Slider{NodeBase: layout.NodeBase{ID: "slider", H: layout.Points(36), W: layout.Grow(1), Box: layout.Box{Background: color.RGBA{R: 34, G: 36, B: 44, A: 255}}}, Value: 0.62},
				layout.Meter{NodeBase: layout.NodeBase{H: layout.Points(18)}, Value: 0.71},
				layout.ScrollBar{NodeBase: layout.NodeBase{ID: "scroll", H: layout.Points(80)}, Offset: 5, Visible: 8, Total: 24},
				layout.Icon{NodeBase: layout.NodeBase{W: layout.Points(28), H: layout.Points(28)}, Name: layout.IconEqualizer, Text: "≋", Animate: true},
				layout.Image{NodeBase: layout.NodeBase{ID: "cover", W: layout.Points(48), H: layout.Points(48), Box: layout.Box{Round: true, Background: color.RGBA{R: 40, G: 40, B: 40, A: 255}}}, Source: s.art, Fit: layout.ImageCover, Text: "cover"},
				layout.Text{NodeBase: layout.NodeBase{W: layout.Points(180), H: layout.Points(32)}, String: "╭──────╮\n│ canvas │\n╰──────╯"},
				layout.Label{NodeBase: layout.NodeBase{Overlay: true, W: layout.Points(90), H: layout.Points(28), Box: layout.Box{Round: true, Background: color.RGBA{R: 45, G: 35, B: 70, A: 255}}}, Text: "overlay layer", Style: layout.TextStyle{Role: layout.TextSubtle}},
			}}
			return flatte.Frame{Root: root, Title: "Canvas smoke"}
		},
	}
}
