package window

import (
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/lunguini/flatte"
)

func TestNativeMediaSelectorsAndMetadataSmoke(t *testing.T) {
	if mediaCommandFailed != 200 || mediaCommandNoActionableItem != 110 {
		t.Fatal("Media command statuses differ from the SDK contract")
	}
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	withPool(func() {
		var delivered []flatte.MediaCommand
		media, err := newNativeMedia(func(command flatte.MediaCommand) { delivered = append(delivered, command) })
		if err != nil {
			t.Fatal(err)
		}
		defer media.detach(false) // Do not claim or clear the user's Now Playing item.
		commands := []flatte.MediaCommand{flatte.MediaPlay, flatte.MediaPause, flatte.MediaTogglePlayPause, flatte.MediaNext, flatte.MediaPrevious}
		for _, command := range commands {
			if !objc.Send[bool](media.target, sel("respondsToSelector:"), media.actions[command]) {
				t.Errorf("target does not respond to selector %v", media.actions[command])
			}
			if objc.Send[bool](media.commands[command], sel("isEnabled")) {
				t.Errorf("remote command %d starts enabled", command)
			}
		}

		// Invoke actual action selectors without enabling system commands
		// or publishing a Now Playing item.
		for _, command := range commands {
			media.mu.Lock()
			media.enabled[command] = true
			media.mu.Unlock()
			status := objc.Send[int](media.target, media.actions[command], objc.ID(0))
			if status != 0 || len(delivered) == 0 || delivered[len(delivered)-1] != command {
				t.Fatalf("selector %d status=%d delivered=%v", command, status, delivered)
			}
		}

		item := &flatte.NowPlaying{
			Title: "Track", Artist: "Artist", Album: "Album",
			Duration: 3*time.Minute + 7*time.Second, Position: 42 * time.Second, Playing: true,
		}
		info := nowPlayingInfo(item, media.keys)
		defer info.Send(sel("release"))
		for _, test := range []struct {
			name string
			key  objc.ID
			want string
		}{{"title", media.keys.title, item.Title}, {"artist", media.keys.artist, item.Artist}, {"album", media.keys.album, item.Album}} {
			got := plainString(info.Send(sel("objectForKey:"), test.key))
			if got != test.want {
				t.Errorf("metadata %s = %q, want %q", test.name, got, test.want)
			}
		}
		if got := objc.Send[float64](info.Send(sel("objectForKey:"), media.keys.duration), sel("doubleValue")); got != 187 {
			t.Errorf("duration metadata = %v, want 187", got)
		}
		if got := objc.Send[float64](info.Send(sel("objectForKey:"), media.keys.elapsed), sel("doubleValue")); got != 42 {
			t.Errorf("position metadata = %v, want 42", got)
		}
		if got := objc.Send[float64](info.Send(sel("objectForKey:"), media.keys.rate), sel("doubleValue")); got != 1 {
			t.Errorf("playback rate = %v, want 1", got)
		}

		oldTarget := media.target
		oldTarget.Send(sel("retain"))
		media.detach(false)
		next, err := newNativeMedia(func(flatte.MediaCommand) {})
		if err != nil {
			oldTarget.Send(sel("release"))
			t.Fatal(err)
		}
		defer next.detach(false)
		next.mu.Lock()
		next.enabled[flatte.MediaPlay] = true
		next.mu.Unlock()
		if got := deliverMediaCommand(oldTarget, flatte.MediaPlay); got != mediaCommandFailed {
			t.Errorf("late command from an old target returned %d, want failure", got)
		}
		oldTarget.Send(sel("release"))
	})
}
