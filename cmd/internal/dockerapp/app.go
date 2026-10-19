package dockerapp

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
	rects         map[string]layout.Rect // full-frame solved geometry, from LayoutEvent
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
	// The host solves the frame tree and delivers the geometry before painting
	// whenever it changes — after a resize, a screen switch, a modal, a drag,
	// or a list whose row count follows its pane. Adopting it here is the only
	// place geometry enters state.
	if layoutEv, ok := ev.(flatte.LayoutEvent); ok {
		s.rects = layoutEv.Rects
		s.adoptScreenRects(s.screen, layoutEv.Rects)
		return
	}
	// A LayoutEvent only arrives when geometry changes. Screens with identical
	// layouts (images ↔ volumes) switch without one, so the newly active
	// screen adopts the last delivered rects — which, unchanged, still
	// describe the frame. A different layout re-adopts via its own event.
	initialScreen := s.screen
	defer func() {
		if s.screen != initialScreen && s.rects != nil {
			s.adoptScreenRects(s.screen, s.rects)
		}
	}()

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

	// Rehydrate data-derived widgets now; the LayoutEvent the host delivers
	// before the next paint sizes them from the solved frame and records hit
	// zones.
	s.containers.afterResize()
}

// frameTree is the one source of truth for the frame: View returns it as the
// Root, and the host's solve of it is what LayoutEvent and mouse targets
// report, so geometry and pixels cannot drift.
func (s *State) frameTree() layout.Node {
	status := func(r layout.Rect) string {
		if s.commandModal != nil {
			return s.commandModal.View(r.W)
		}
		return s.containers.renderStatusLine(r.W)
	}

	children := []layout.Node{
		headerNode(s),
		s.activeBody(status),
		Separator{},
		Footer{State: s},
	}
	if s.confirmModal != nil {
		children = append(children, modalNode(s.confirmModal))
	}
	return layout.Col{Children: children}
}

// activeBody returns the active screen's body subtree, ready to slot into the
// frame tree as real children (so the single frame solve owns its geometry).
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

func (s *State) adoptScreenRects(sc screen, rects map[string]layout.Rect) {
	switch sc {
	case screenContainers:
		s.containers.adopt(rects)
	case screenImages:
		s.images.adopt(rects)
	case screenVolumes:
		s.volumes.adopt(rects)
	}
}

func View(s *State, _ flatte.RenderContext) flatte.Frame {
	// The host solves and paints the whole frame — header, panes, dividers,
	// status, footer — in one pass at the terminal size.
	frame := flatte.Frame{
		Root:  s.frameTree(),
		Title: "flat-docker — " + s.screen.Name(),
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
