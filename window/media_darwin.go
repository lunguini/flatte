package window

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/lunguini/flatte"
)

const mediaPlayerFramework = "/System/Library/Frameworks/MediaPlayer.framework/MediaPlayer"

const (
	mediaStatePlaying            = 1
	mediaStatePaused             = 2
	mediaStateStopped            = 3
	mediaCommandOK               = 0
	mediaCommandNoActionableItem = 110
	mediaCommandFailed           = 200
)

var (
	mediaFrameworkOnce sync.Once
	mediaFrameworkErr  error
	mediaFramework     uintptr

	mediaTargetOnce  sync.Once
	mediaTargetClass objc.Class
	mediaTargetErr   error

	mediaCurrentMu sync.Mutex
	mediaCurrent   *nativeMedia
)

type nowPlayingKeys struct {
	title, artist, album, duration, elapsed, rate objc.ID
}

// nativeMedia is owned by one macOS window host and used on its UI thread.
// Only command eligibility and lifecycle are read from the remote callback.
type nativeMedia struct {
	mu         sync.Mutex
	infoCenter objc.ID
	target     objc.ID
	keys       nowPlayingKeys
	commands   map[flatte.MediaCommand]objc.ID
	actions    map[flatte.MediaCommand]objc.SEL
	enabled    map[flatte.MediaCommand]bool
	send       func(flatte.MediaCommand)
	last       *flatte.NowPlaying
	updated    bool
	closed     bool
}

func loadMediaPlayer() error {
	mediaFrameworkOnce.Do(func() {
		mediaFramework, mediaFrameworkErr = purego.Dlopen(mediaPlayerFramework, purego.RTLD_GLOBAL|purego.RTLD_LAZY)
		if mediaFrameworkErr != nil {
			mediaFrameworkErr = fmt.Errorf("window: load MediaPlayer: %w", mediaFrameworkErr)
		}
	})
	return mediaFrameworkErr
}

// loadNowPlayingKeys resolves the framework's exported NSString constants.
// Keeping the values from MediaPlayer avoids relying on undocumented literal
// keys in the Now Playing dictionary.
func loadNowPlayingKeys() (nowPlayingKeys, error) {
	if err := loadMediaPlayer(); err != nil {
		return nowPlayingKeys{}, err
	}
	stringClass := objc.GetClass("NSString")
	if stringClass == 0 {
		return nowPlayingKeys{}, errors.New("window: NSString class unavailable")
	}
	get := func(name string) (objc.ID, error) {
		value, err := constant(mediaFramework, name)
		if err != nil {
			return 0, fmt.Errorf("window: MediaPlayer constant %s: %w", name, err)
		}
		if value == 0 || !objc.Send[bool](value, sel("isKindOfClass:"), stringClass) {
			return 0, fmt.Errorf("window: MediaPlayer constant %s is not an NSString", name)
		}
		return value, nil
	}
	keys := nowPlayingKeys{}
	fields := []struct {
		name string
		to   *objc.ID
	}{
		{"MPMediaItemPropertyTitle", &keys.title},
		{"MPMediaItemPropertyArtist", &keys.artist},
		{"MPMediaItemPropertyAlbumTitle", &keys.album},
		{"MPMediaItemPropertyPlaybackDuration", &keys.duration},
		{"MPNowPlayingInfoPropertyElapsedPlaybackTime", &keys.elapsed},
		{"MPNowPlayingInfoPropertyPlaybackRate", &keys.rate},
	}
	for _, field := range fields {
		value, err := get(field.name)
		if err != nil {
			return nowPlayingKeys{}, err
		}
		*field.to = value
	}
	return keys, nil
}

func newNativeMedia(send func(flatte.MediaCommand)) (*nativeMedia, error) {
	if send == nil {
		return nil, errors.New("window: media command callback is nil")
	}
	keys, err := loadNowPlayingKeys()
	if err != nil {
		return nil, err
	}
	mediaTargetOnce.Do(func() {
		mediaTargetClass, mediaTargetErr = objc.RegisterClass("FlatteMediaCommandTarget", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
			{Cmd: sel("flattePlay:"), Fn: func(target objc.ID, _ objc.SEL, _ objc.ID) int { return deliverMediaCommand(target, flatte.MediaPlay) }},
			{Cmd: sel("flattePause:"), Fn: func(target objc.ID, _ objc.SEL, _ objc.ID) int { return deliverMediaCommand(target, flatte.MediaPause) }},
			{Cmd: sel("flatteTogglePlayPause:"), Fn: func(target objc.ID, _ objc.SEL, _ objc.ID) int {
				return deliverMediaCommand(target, flatte.MediaTogglePlayPause)
			}},
			{Cmd: sel("flatteNext:"), Fn: func(target objc.ID, _ objc.SEL, _ objc.ID) int { return deliverMediaCommand(target, flatte.MediaNext) }},
			{Cmd: sel("flattePrevious:"), Fn: func(target objc.ID, _ objc.SEL, _ objc.ID) int {
				return deliverMediaCommand(target, flatte.MediaPrevious)
			}},
		})
	})
	if mediaTargetErr != nil {
		return nil, fmt.Errorf("window: register media command target: %w", mediaTargetErr)
	}
	centerClass := objc.GetClass("MPRemoteCommandCenter")
	infoClass := objc.GetClass("MPNowPlayingInfoCenter")
	if centerClass == 0 || infoClass == 0 {
		return nil, errors.New("window: MediaPlayer classes unavailable")
	}
	center := objc.ID(centerClass).Send(sel("sharedCommandCenter"))
	infoCenter := objc.ID(infoClass).Send(sel("defaultCenter"))
	if center == 0 || infoCenter == 0 {
		return nil, errors.New("window: MediaPlayer centers unavailable")
	}
	target := objc.ID(mediaTargetClass).Send(sel("new"))
	if target == 0 {
		return nil, errors.New("window: create media command target")
	}
	m := &nativeMedia{
		infoCenter: infoCenter, target: target, keys: keys, send: send,
		commands: map[flatte.MediaCommand]objc.ID{
			flatte.MediaPlay:            center.Send(sel("playCommand")),
			flatte.MediaPause:           center.Send(sel("pauseCommand")),
			flatte.MediaTogglePlayPause: center.Send(sel("togglePlayPauseCommand")),
			flatte.MediaNext:            center.Send(sel("nextTrackCommand")),
			flatte.MediaPrevious:        center.Send(sel("previousTrackCommand")),
		},
		actions: map[flatte.MediaCommand]objc.SEL{
			flatte.MediaPlay:            sel("flattePlay:"),
			flatte.MediaPause:           sel("flattePause:"),
			flatte.MediaTogglePlayPause: sel("flatteTogglePlayPause:"),
			flatte.MediaNext:            sel("flatteNext:"),
			flatte.MediaPrevious:        sel("flattePrevious:"),
		},
		enabled: make(map[flatte.MediaCommand]bool),
	}
	for command, remote := range m.commands {
		if remote == 0 {
			target.Send(sel("release"))
			return nil, fmt.Errorf("window: MediaPlayer command %d unavailable", command)
		}
	}
	mediaCurrentMu.Lock()
	if mediaCurrent != nil {
		mediaCurrentMu.Unlock()
		target.Send(sel("release"))
		return nil, errors.New("window: media controls already owned")
	}
	mediaCurrent = m
	mediaCurrentMu.Unlock()
	for command, remote := range m.commands {
		remote.Send(sel("setEnabled:"), false)
		remote.Send(sel("addTarget:action:"), target, m.actions[command])
	}
	return m, nil
}

func deliverMediaCommand(target objc.ID, command flatte.MediaCommand) int {
	mediaCurrentMu.Lock()
	m := mediaCurrent
	mediaCurrentMu.Unlock()
	if m == nil {
		return mediaCommandNoActionableItem
	}
	m.mu.Lock()
	if target == 0 || target != m.target {
		m.mu.Unlock()
		return mediaCommandFailed
	}
	if m.closed || !m.enabled[command] {
		m.mu.Unlock()
		return mediaCommandNoActionableItem
	}
	send := m.send
	m.mu.Unlock()
	if send == nil {
		return mediaCommandFailed
	}
	send(command) // The callback only posts to the host's UI loop.
	return mediaCommandOK
}

func (m *nativeMedia) update(item *flatte.NowPlaying) {
	m.mu.Lock()
	if m.closed || m.updated && sameNowPlaying(m.last, item) {
		m.mu.Unlock()
		return
	}
	stateChanged := !m.updated || (m.last == nil) != (item == nil) || m.last != nil && item != nil && m.last.Playing != item.Playing
	states := mediaCommandStates(item)
	changed := make([]flatte.MediaCommand, 0, len(states))
	for command, enabled := range states {
		if !m.updated || m.enabled[command] != enabled {
			changed = append(changed, command)
		}
		m.enabled[command] = enabled
	}
	if item == nil {
		m.last = nil
	} else {
		copy := *item
		m.last = &copy
	}
	m.updated = true
	m.mu.Unlock()

	if item == nil {
		m.infoCenter.Send(sel("setNowPlayingInfo:"), objc.ID(0))
		if stateChanged {
			m.infoCenter.Send(sel("setPlaybackState:"), mediaStateStopped)
		}
		m.setCommandEnabled(changed, states)
		return
	}
	info := nowPlayingInfo(item, m.keys)
	m.infoCenter.Send(sel("setNowPlayingInfo:"), info)
	info.Send(sel("release"))
	if stateChanged {
		state := mediaStatePaused
		if item.Playing {
			state = mediaStatePlaying
		}
		m.infoCenter.Send(sel("setPlaybackState:"), state)
	}
	m.setCommandEnabled(changed, states)
}

// nowPlayingInfo builds a retained NSDictionary without publishing it. The
// caller releases it after assigning or inspecting the metadata.
func nowPlayingInfo(item *flatte.NowPlaying, keys nowPlayingKeys) objc.ID {
	info := alloc("NSMutableDictionary").Send(sel("init"))
	setText := func(key objc.ID, value string) {
		if value != "" {
			info.Send(sel("setObject:forKey:"), nsString(value), key)
		}
	}
	setNumber := func(key objc.ID, value float64) {
		info.Send(sel("setObject:forKey:"), class("NSNumber").Send(sel("numberWithDouble:"), value), key)
	}
	setText(keys.title, item.Title)
	setText(keys.artist, item.Artist)
	setText(keys.album, item.Album)
	if item.Duration > 0 {
		setNumber(keys.duration, item.Duration.Seconds())
	}
	position := max(item.Position, 0)
	if item.Duration > 0 && position > item.Duration {
		position = item.Duration
	}
	setNumber(keys.elapsed, position.Seconds())
	rate := 0.0
	if item.Playing {
		rate = 1
	}
	setNumber(keys.rate, rate)
	return info
}

func sameNowPlaying(a, b *flatte.NowPlaying) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func mediaCommandStates(item *flatte.NowPlaying) map[flatte.MediaCommand]bool {
	states := map[flatte.MediaCommand]bool{}
	for _, command := range []flatte.MediaCommand{flatte.MediaPlay, flatte.MediaPause, flatte.MediaTogglePlayPause, flatte.MediaNext, flatte.MediaPrevious} {
		enabled := false
		if item != nil {
			switch command {
			case flatte.MediaPlay:
				enabled = !item.Playing
			case flatte.MediaPause:
				enabled = item.Playing
			case flatte.MediaTogglePlayPause:
				enabled = true
			case flatte.MediaNext:
				enabled = item.CanNext
			case flatte.MediaPrevious:
				enabled = item.CanPrevious
			}
		}
		states[command] = enabled
	}
	return states
}

func (m *nativeMedia) setCommandEnabled(changed []flatte.MediaCommand, states map[flatte.MediaCommand]bool) {
	for _, command := range changed {
		m.commands[command].Send(sel("setEnabled:"), states[command])
	}
}

func (m *nativeMedia) close() { m.detach(true) }

// detach removes only this backend's command targets. Tests use clearInfo=false
// so they never claim or clear the user's current Now Playing item.
func (m *nativeMedia) detach(clearInfo bool) {
	mediaCurrentMu.Lock()
	if mediaCurrent == m {
		mediaCurrent = nil
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		mediaCurrentMu.Unlock()
		return
	}
	m.closed = true
	target := m.target
	m.target = 0
	for command := range m.enabled {
		m.enabled[command] = false
	}
	m.last = nil
	m.mu.Unlock()
	mediaCurrentMu.Unlock()

	for command, remote := range m.commands {
		remote.Send(sel("setEnabled:"), false)
		remote.Send(sel("removeTarget:action:"), target, m.actions[command])
	}
	if clearInfo {
		m.infoCenter.Send(sel("setNowPlayingInfo:"), objc.ID(0))
		m.infoCenter.Send(sel("setPlaybackState:"), mediaStateStopped)
	}
	if target != 0 {
		target.Send(sel("release"))
	}
}
