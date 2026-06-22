package flatest

import "github.com/lunguini/flatte"

// Step is one entry in a recorded session: either an event the loop
// received or an update that was applied.
type Step struct {
	Event  flatte.Event // non-nil for an event step
	Update string       // non-empty for an update step
}

// Recorder is a flatte.Tracer that captures the ordered event/update
// stream of a session for replay assertions. Use it as App.Tracer.
type Recorder struct {
	Steps []Step
}

func (r *Recorder) Event(ev flatte.Event) { r.Steps = append(r.Steps, Step{Event: ev}) }
func (r *Recorder) Update(name string)    { r.Steps = append(r.Steps, Step{Update: name}) }

// Updates returns the recorded update names in order.
func (r *Recorder) Updates() []string {
	var names []string
	for _, s := range r.Steps {
		if s.Update != "" {
			names = append(names, s.Update)
		}
	}
	return names
}

// Events returns the recorded events in order.
func (r *Recorder) Events() []flatte.Event {
	var events []flatte.Event
	for _, s := range r.Steps {
		if s.Event != nil {
			events = append(events, s.Event)
		}
	}
	return events
}

// Replay re-drives the recorded EVENTS through a fresh Driver — settling
// after each so any async those events trigger lands — and returns the
// new recording. A test asserts the two update-name streams match: a
// regression lock on event→update determinism.
//
// Limitations (honest): updates are closures and are not serialized, so
// only their names are compared. Settle/Advance cadence is NOT captured,
// so a session that batches several events before settling (e.g. a Latest
// supersede) will not reproduce identically under per-event settling —
// drive those with the Driver directly. ResizeEvents are skipped: the
// fresh Driver delivers its own initial resize at the fixed width.
func Replay[S any](app flatte.App[S], width int, rec *Recorder) *Recorder {
	out := &Recorder{}
	app.Tracer = out
	d := Start(app, width)
	for _, ev := range rec.Events() {
		if _, isResize := ev.(flatte.ResizeEvent); isResize {
			continue
		}
		d.Send(ev)
		d.Settle()
	}
	return out
}
