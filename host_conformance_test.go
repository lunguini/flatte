package flatte_test

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/internal/hostconformance"
)

func TestTerminalHostConformance(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Known bug: with piped input, the key sent after Exec returns is not
		// delivered on Windows. Tracked in the status tracker.
		t.Skip("piped input after Exec is not delivered on Windows")
	}
	want := hostconformance.Expectations{
		Capabilities: flatte.Capabilities{
			Exec: true, Suspend: true, Print: true,
			Alert: true, OpenFile: true, SaveFile: true,
			Notifications: true, Clipboard: true, Mouse: true, Quit: true,
		},
		AlertButton:   1,
		ClipboardRead: false,
		CtrlCExitsRun: true,
	}
	if err := hostconformance.Verify(terminalConformanceAdapter{}, want); err != nil {
		t.Fatal(err)
	}
}

type terminalConformanceAdapter struct{}

func (terminalConformanceAdapter) Start(app flatte.App[hostconformance.State], state *hostconformance.State, observer *hostconformance.Observer) (hostconformance.Driver, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	d := &terminalConformanceDriver{state: state, observer: observer, input: writer, output: &lockedBytes{}, cancel: cancel, done: make(chan error, 1)}
	go func() {
		d.done <- flatte.Run(ctx, app, flatte.WithInput(reader), flatte.WithOutput(d.output), flatte.WithMouse(flatte.MouseModeCellMotion))
	}()
	return d, nil
}

type terminalConformanceDriver struct {
	state    *hostconformance.State
	observer *hostconformance.Observer
	input    *os.File
	output   *lockedBytes
	cancel   context.CancelFunc
	done     chan error
	finished atomic.Bool
	mu       sync.Mutex
	runErr   error
}

func (d *terminalConformanceDriver) Send(ev flatte.KeyEvent) error {
	var b []byte
	switch ev.Key {
	case flatte.KeyCtrlC:
		b = []byte{3}
	case flatte.KeyEnter:
		b = []byte{'\r'}
	case flatte.KeyTab:
		b = []byte{'\t'}
	case flatte.KeyEscape:
		b = []byte{0x1b}
	case flatte.KeyCharacter:
		b = []byte(string(ev.Rune))
	default:
		return fmt.Errorf("terminal adapter cannot encode key %+v", ev)
	}
	_, err := d.input.Write(b)
	return err
}

func (d *terminalConformanceDriver) AnswerAlert(button int) error {
	if button == 1 {
		if err := d.Send(flatte.KeyEvent{Key: flatte.KeyTab}); err != nil {
			return err
		}
	}
	return d.Send(flatte.KeyEvent{Key: flatte.KeyEnter})
}

func (d *terminalConformanceDriver) Wait(ready func(hostconformance.Snapshot) bool) error {
	err := hostconformance.WaitUntil(5*time.Second, func() bool {
		if ready(d.state.Read()) {
			return true
		}
		d.pollDone()
		return false
	})
	if err != nil {
		d.pollDone()
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.runErr != nil {
			return fmt.Errorf("Run returned early: %w", d.runErr)
		}
		return err
	}
	return nil
}

func (d *terminalConformanceDriver) Finished() bool {
	d.pollDone()
	return d.finished.Load()
}

func (d *terminalConformanceDriver) pollDone() {
	select {
	case err := <-d.done:
		d.mu.Lock()
		d.runErr = err
		d.mu.Unlock()
		d.finished.Store(true)
	default:
	}
}

func (d *terminalConformanceDriver) ClipboardWriteObserved() bool {
	text := d.output.String()
	return strings.Contains(text, ansi.SetSystemClipboard(hostconformance.ClipboardText)) &&
		strings.Contains(text, ansi.RequestSystemClipboard)
}

func (d *terminalConformanceDriver) Close() {
	d.cancel()
	_ = d.input.Close()
	if !d.Finished() {
		select {
		case err := <-d.done:
			d.mu.Lock()
			d.runErr = err
			d.mu.Unlock()
			d.finished.Store(true)
		case <-time.After(5 * time.Second):
		}
	}
}

type lockedBytes struct {
	mu sync.Mutex
	b  strings.Builder
}

func (b *lockedBytes) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBytes) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}
