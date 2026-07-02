package main

import (
	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

type screen int

const (
	screenContainers screen = iota
	screenImages
	screenVolumes
)

func (sc screen) Name() string {
	switch sc {
	case screenContainers:
		return "containers"
	case screenImages:
		return "images"
	case screenVolumes:
		return "volumes"
	}
	return "unknown"
}

type State struct {
	width, height int
	screen        screen
	rects         map[string]layout.Rect // full-frame solved geometry
	bodyYOffset   int                    // Y where body starts (header + separator)

	containers containersScreen
	images     *imagesScreen
	volumes    *volumesScreen

	confirmModal *confirmModel
	commandModal *commandModel

	headerTabs   *flatui.TabBar
	headerBorder flatui.TabBarBorder

	cmdHistory []string
}

func NewState() *State {
	s := &State{screen: screenContainers}
	s.containers = newContainersScreen()
	s.images = newImagesScreen()
	s.volumes = newVolumesScreen()

	s.headerBorder = flatui.TabBarBorder{
		Color:  pal.accent,
		Bottom: true,
	}
	s.headerTabs = flatui.NewTabBar(
		flatui.TabItem{ID: "containers", Label: "1 containers"},
		flatui.TabItem{ID: "images", Label: "2 images"},
		flatui.TabItem{ID: "volumes", Label: "3 volumes"},
	).WithGlyphs(pickGlyphs()).WithColors(pal.accent, pal.tabBg, pal.bg).WithID("header")
	return s
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	if s.confirmModal != nil {
		s.confirmModal.Handle(s, ev, fx)
		return
	}
	if s.commandModal != nil {
		s.commandModal.Handle(s, ev, fx)
		return
	}

	// Header tab mouse clicks — root-level, above screen dispatch. The header
	// is a real subtree of the frame, so its per-tab rects live in s.rects;
	// HitTest maps the click to a tab index with no coordinate math here.
	if m, ok := ev.(flatte.MouseEvent); ok && m.Action == flatte.MousePress {
		if idx, ok := s.headerTabs.HitTest(s.rects, m.X, m.Y); ok {
			s.screen = screen(idx)
			s.headerTabs.SetActive(idx)
			return
		}
	}

	switch e := ev.(type) {
	case flatte.ResizeEvent:
		s.resize(e.Width, e.Height)
		return
	case flatte.KeyEvent:
		if handleGlobalKey(s, e, fx) {
			s.headerTabs.SetActive(int(s.screen))
			return
		}
	}

	switch s.screen {
	case screenContainers:
		s.containers.Handle(s, ev, fx)
	case screenImages:
		s.images.Handle(s, ev, fx)
	case screenVolumes:
		s.volumes.Handle(s, ev, fx)
	}
}

func handleGlobalKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) bool {
	if key.Key != flatte.KeyCharacter {
		return false
	}
	switch key.Rune {
	case '1':
		s.screen = screenContainers
		return true
	case '2':
		s.screen = screenImages
		return true
	case '3':
		s.screen = screenVolumes
		return true
	case 'q', 'Q':
		fx.Quit()
		return true
	}
	return false
}

// bodyRect returns the absolute rectangle the body occupies within the frame:
// full width, starting just below the header, minus the bottom chrome rows.
func (s *State) bodyRect() layout.Rect {
	return layout.Rect{
		Y: s.bodyYOffset,
		W: s.width,
		H: max(s.height-s.bodyYOffset-chromeRowsBottom, 0),
	}
}

func (s *State) resize(width, height int) {
	s.width, s.height = width, height
	s.headerTabs.SetActive(int(s.screen))

	// Header height = content row + enabled border rules.
	s.bodyYOffset = headerHeight(s)

	body := s.bodyRect()
	s.containers.initLayout(body.W, body.H, s.bodyYOffset)
	s.images.initLayout(body.W, body.H, s.bodyYOffset)
	s.volumes.initLayout(body.W, body.H, s.bodyYOffset)

	// Warm every screen's geometry from one absolute solve apiece so hit-testing
	// works before the first View and for screens that are not yet active.
	s.warmRects(body)
	s.containers.afterResize()
}

// warmRects solves each screen's body subtree at the absolute body rect and
// hands the resulting geometry to that screen. Cold path (resize only) — the
// per-frame path resolves the active screen as part of the single frame solve
// in View.
func (s *State) warmRects(body layout.Rect) {
	s.containers.adopt(solveScreenRects(s.containers.bodyNode(nil), body))
	s.images.adopt(solveScreenRects(s.images.bodyNode(), body))
	s.volumes.adopt(solveScreenRects(s.volumes.bodyNode(), body))
}

// refreshActiveRects re-solves only the active screen's geometry. Used by
// divider drags, which change pane widths between frames and need the updated
// rects immediately (before the next View).
func (s *State) refreshActiveRects() {
	body := s.bodyRect()
	switch s.screen {
	case screenContainers:
		s.containers.adopt(solveScreenRects(s.containers.bodyNode(nil), body))
	case screenImages:
		s.images.adopt(solveScreenRects(s.images.bodyNode(), body))
	case screenVolumes:
		s.volumes.adopt(solveScreenRects(s.volumes.bodyNode(), body))
	}
}

// activeBody returns the active screen's body subtree, ready to slot into the
// frame tree as real children (so the single frame Solve owns its geometry).
func (s *State) activeBody(status func(layout.Rect) string) layout.Node {
	switch s.screen {
	case screenImages:
		return s.images.bodyNode()
	case screenVolumes:
		return s.volumes.bodyNode()
	default:
		return s.containers.bodyNode(status)
	}
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	width := ctx.Width
	if s.width > 0 {
		width = s.width
	}
	height := s.height
	if height <= 0 {
		height = 24
	}
	if s.width != width || s.height != height {
		s.resize(width, height)
	}

	status := func(r layout.Rect) string {
		if s.commandModal != nil {
			return s.commandModal.View(r.W)
		}
		return s.containers.renderStatusLine(r.W)
	}

	tree := layout.Col{Children: []layout.Node{
		headerNode(s),
		s.activeBody(status),
		Separator{},
		Footer{State: s},
	}}

	// One top-to-bottom pass resolves the whole frame — header, panes,
	// dividers, status, footer — in absolute coordinates AND paints it into a
	// cell buffer. The active screen adopts that geometry for hit-testing and
	// widget sizing; solve and render can't drift because there is one walk.
	content, rects := layout.SolveAndCompose(tree, width, height)
	s.rects = rects
	switch s.screen {
	case screenContainers:
		s.containers.adopt(s.rects)
	case screenImages:
		s.images.adopt(s.rects)
	case screenVolumes:
		s.volumes.adopt(s.rects)
	}

	// Modal overlay (composited via ultraviolet for proper ANSI handling).
	if s.confirmModal != nil {
		content = flatui.Overlay(content, renderModal(s.confirmModal))
	}

	frame := flatte.Frame{
		Content: content,
		Title:   "flat-docker — " + s.screen.Name(),
	}
	if s.commandModal != nil {
		if r, ok := s.rects["status"]; ok {
			frame.Cursor = &flatte.Cursor{
				X: r.X + s.commandModal.cursorFrameX(),
				Y: r.Y,
			}
		}
	}
	return frame
}
