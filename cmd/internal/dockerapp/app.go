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
	rects         map[string]layout.Rect // full-frame solved geometry
	bodyYOffset   int                    // Y where body starts (header + separator)
	geometryDirty bool
	warmInactive  bool
	reflowCount   uint64 // test/benchmark instrumentation; not app state

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
	// A prior event normally leaves geometry prepared. This check also makes
	// direct callers safe when they mutate state between events.
	s.reflow()
	initialScreen := s.screen
	initialConfirm, initialCommand := s.confirmModal, s.commandModal
	defer func() {
		// Modal presence changes the frame tree. Screen changes require warming
		// the newly active screen (and all peers, which may have stale widths).
		if initialConfirm != s.confirmModal || initialCommand != s.commandModal {
			s.markGeometryDirty(false)
		}
		if initialScreen != s.screen {
			s.markGeometryDirty(true)
		}
		// Geometry is event-time state. View remains a read-only projection of
		// the prepared state, while ignored events avoid a layout solve.
		s.reflow()
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
	s.markGeometryDirty(true)
	s.headerTabs.SetActive(int(s.screen))

	// Header height = content row + enabled border rules.
	s.bodyYOffset = headerHeight(s)

	body := s.bodyRect()
	s.containers.initLayout(body.W, body.H, s.bodyYOffset)
	s.images.initLayout(body.W, body.H, s.bodyYOffset)
	s.volumes.initLayout(body.W, body.H, s.bodyYOffset)

	// Rehydrate data-derived widgets before the first solve. The subsequent
	// reflow sizes them from the full frame geometry and records hit zones.
	s.containers.afterResize()
}

// refreshActiveRects is kept as the divider-drag hook, but the refreshed map
// is always the full frame map. A body-only solve would leave header, footer,
// modal, and active widget geometry out of sync until the next render.
func (s *State) refreshActiveRects() {
	s.markGeometryDirty(false)
}

func (s *State) markGeometryDirty(warmInactive bool) {
	s.geometryDirty = true
	s.warmInactive = s.warmInactive || warmInactive
}

// frameTree is the one source of truth for the active frame in both
// event-time geometry and rendering. Keeping this builder shared prevents a
// resize/drag solve from silently diverging from what View composes.
func (s *State) frameTree() layout.Node {
	return s.frameTreeFor(s.screen)
}

// frameTreeFor lets the resize path prepare each screen's widgets before a
// user switches to it. The active frame and View still use exactly the same
// tree builder; inactive trees are solved only to warm their event geometry.
func (s *State) frameTreeFor(sc screen) layout.Node {
	status := func(r layout.Rect) string {
		if s.commandModal != nil {
			return s.commandModal.View(r.W)
		}
		return s.containers.renderStatusLine(r.W)
	}

	children := []layout.Node{
		headerNode(s),
		s.bodyFor(sc, status),
		Separator{},
		Footer{State: s},
	}
	if s.confirmModal != nil {
		children = append(children, modalNode(s.confirmModal))
	}
	return layout.Col{Children: children}
}

// activeBody returns the active screen's body subtree, ready to slot into the
// frame tree as real children (so the single frame Solve owns its geometry).
func (s *State) activeBody(status func(layout.Rect) string) layout.Node {
	return s.bodyFor(s.screen, status)
}

func (s *State) bodyFor(sc screen, status func(layout.Rect) string) layout.Node {
	switch sc {
	case screenImages:
		return s.images.bodyNode()
	case screenVolumes:
		return s.volumes.bodyNode()
	default:
		return s.containers.bodyNode(status)
	}
}

// reflow prepares all geometry needed by the next event and frame. It runs on
// the loop goroutine, after mutations, and never paints. A list's visible row
// count is itself derived from its solved pane height, so solve/adopt once
// more when that count changes to make row rects available immediately after
// resize or a screen switch.
func (s *State) reflow() {
	if (!s.geometryDirty && !s.containers.geometryDirty) || s.width <= 0 || s.height <= 0 {
		return
	}
	s.reflowCount++
	warmInactive := s.warmInactive
	for pass := 0; pass < 2; pass++ {
		oldContainersHeight := s.containers.listHeight
		oldImagesHeight := s.images.list.Height()
		oldVolumesHeight := s.volumes.list.Height()
		screens := []screen{s.screen}
		if warmInactive {
			screens = []screen{screenContainers, screenImages, screenVolumes}
		}
		for _, sc := range screens {
			rects := layout.Solve(s.frameTreeFor(sc), s.width, s.height)
			if sc == s.screen {
				s.rects = rects
			}
			s.adoptScreenRects(sc, rects)
		}
		if oldContainersHeight == s.containers.listHeight &&
			oldImagesHeight == s.images.list.Height() &&
			oldVolumesHeight == s.volumes.list.Height() {
			s.geometryDirty = false
			s.warmInactive = false
			return
		}
	}
	s.geometryDirty = false
	s.warmInactive = false
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

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	width := ctx.Width
	if s.width > 0 {
		width = s.width
	}
	height := s.height
	if height <= 0 {
		height = 24
	}
	if width <= 0 || height <= 0 {
		return flatte.Frame{Title: "flat-docker — " + s.screen.Name()}
	}

	// One top-to-bottom pass resolves the whole frame — header, panes,
	// dividers, status, footer — and paints it into a cell buffer. Geometry is
	// prepared by Handle; this compose pass deliberately does not adopt it.
	content, _ := layout.SolveAndCompose(s.frameTree(), width, height)

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
