package window

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/internal/hostconformance"
)

func TestWindowHostConformance(t *testing.T) {
	want := hostconformance.Expectations{
		Capabilities: flatte.Capabilities{
			Alert: true, OpenFile: true, SaveFile: true,
			Notifications: true, Clipboard: true, Drop: true, Mouse: true, Quit: true,
		},
		AlertButton:    1,
		ExecError:      "unsupported",
		ClipboardRead:  true,
		ClipboardValue: hostconformance.ClipboardText,
		CtrlCDelivered: true,
		CtrlCExitsRun:  false,
	}
	if err := hostconformance.Verify(windowConformanceAdapter{}, want); err != nil {
		t.Fatal(err)
	}
}

type windowConformanceAdapter struct{}

func (windowConformanceAdapter) Start(app flatte.App[hostconformance.State], state *hostconformance.State, observer *hostconformance.Observer) (hostconformance.Driver, error) {
	b := &fakeBackend{}
	d := &windowConformanceDriver{state: state, observer: observer, backend: b}
	d.host = newHost(context.Background(), app, cellMetrics, b.post, func() { d.finished = true }, func() {})
	d.host.clip = &fakeClip{}
	d.host.dialogs = &fakeDialogs{answer: flatte.DialogResult{Button: 1}}
	d.host.notes = &fakeNotifier{}
	d.host.start(800, 400)
	d.host.render()
	return d, nil
}

type windowConformanceDriver struct {
	state    *hostconformance.State
	observer *hostconformance.Observer
	backend  *fakeBackend
	host     *host[hostconformance.State]
	finished bool
	once     sync.Once
}

func (d *windowConformanceDriver) Send(ev flatte.KeyEvent) error {
	d.host.key(ev)
	if d.host.quitting {
		d.Finish()
	}
	return nil
}

func (d *windowConformanceDriver) AnswerAlert(int) error { return nil }

func (d *windowConformanceDriver) Wait(ready func(hostconformance.Snapshot) bool) error {
	return hostconformance.WaitUntil(5*time.Second, func() bool {
		d.drain()
		return ready(d.state.Read())
	})
}

func (d *windowConformanceDriver) Finished() bool { return d.finished }

func (d *windowConformanceDriver) ClipboardWriteObserved() bool {
	clip, ok := d.host.clip.(*fakeClip)
	return ok && clip.text == hostconformance.ClipboardText
}

func (d *windowConformanceDriver) Close() { d.Finish() }

func (d *windowConformanceDriver) Finish() {
	d.once.Do(func() {
		d.host.close()
		d.finished = true
	})
}

func (d *windowConformanceDriver) drain() {
	d.backend.mu.Lock()
	fs := d.backend.posted
	d.backend.posted = nil
	d.backend.mu.Unlock()
	for _, f := range fs {
		f()
	}
}
