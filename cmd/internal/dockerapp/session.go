package dockerapp

const sessionFile = ".flat-docker-state.gob"

// SessionState is the gob-serializable subset of State that survives
// restart: active screen, cursor positions, pane widths, filter, command
// history. Widget objects (List, Viewport, TabBar, etc.) are not serialized —
// they are recreated fresh on boot and rehydrated from this struct.
type SessionState struct {
	Screen          int
	ContainerCursor int
	ContainerFilter string
	ContainerTab    int
	ContainerListW  int
	ContainerActW   int
	ImageCursor     int
	ImageListW      int
	VolumeCursor    int
	VolumeListW     int
	CmdHistory      []string
}

func (s *State) toSession() SessionState {
	return SessionState{
		Screen:          int(s.screen),
		ContainerCursor: s.containers.list.Cursor(),
		ContainerFilter: s.containers.filter.Value,
		ContainerTab:    int(s.containers.tab),
		ContainerListW:  s.containers.listPaneWidth,
		ContainerActW:   s.containers.activityPaneWidth,
		ImageCursor:     s.images.list.Cursor(),
		ImageListW:      s.images.listPaneWidth,
		VolumeCursor:    s.volumes.list.Cursor(),
		VolumeListW:     s.volumes.listPaneWidth,
		CmdHistory:      s.cmdHistory,
	}
}

func newStateFromSession(ss SessionState) *State {
	s := NewState()
	s.screen = screen(ss.Screen)
	s.cmdHistory = ss.CmdHistory
	if ss.ContainerListW > 0 {
		s.containers.listPaneWidth = ss.ContainerListW
	}
	if ss.ContainerActW > 0 {
		s.containers.activityPaneWidth = ss.ContainerActW
	}
	s.containers.filter.Value = ss.ContainerFilter
	s.containers.tab = detailTab(ss.ContainerTab)
	if ss.ContainerFilter != "" {
		s.containers.focus.Select(focusList)
	}
	if ss.ImageListW > 0 {
		s.images.listPaneWidth = ss.ImageListW
	}
	if ss.VolumeListW > 0 {
		s.volumes.listPaneWidth = ss.VolumeListW
	}
	// Defer cursor restoration until after layout (list count is set then).
	if ss.ContainerCursor >= 0 {
		s.containers.pendingCursor = ss.ContainerCursor
	}
	if ss.ImageCursor >= 0 {
		s.images.list.Select(ss.ImageCursor)
	}
	if ss.VolumeCursor >= 0 {
		s.volumes.list.Select(ss.VolumeCursor)
	}
	return s
}
