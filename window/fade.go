package window

import "time"

// How long hover and press take to fade in or out.
const (
	hoverFade = 140 * time.Millisecond
	pressFade = 80 * time.Millisecond
)

// fader is a level from 0 to 1 per node ID (hovered, pressed), easing to
// on or off over d from wherever it was when it last changed.
type fader struct {
	d     time.Duration
	fades map[string]fade
}

type fade struct {
	on   bool
	at   time.Time
	from float64
}

func newFader(d time.Duration) *fader { return &fader{d: d, fades: map[string]fade{}} }

// set turns id on or off at now.
func (f *fader) set(id string, on bool, now time.Time) {
	cur, ok := f.fades[id]
	if ok && cur.on == on || !ok && !on {
		return
	}
	f.fades[id] = fade{on: on, at: now, from: f.level(id, now)}
}

// only turns on the ids in on and off all others, at now.
func (f *fader) only(on map[string]bool, now time.Time) {
	for id := range f.fades {
		if !on[id] {
			f.set(id, false, now)
		}
	}
	for id := range on {
		f.set(id, true, now)
	}
}

// level is id's level at now; a faded-out id is forgotten.
func (f *fader) level(id string, now time.Time) float64 {
	cur, ok := f.fades[id]
	if !ok {
		return 0
	}
	t := 1.0
	if f.d > 0 {
		t = min(max(float64(now.Sub(cur.at))/float64(f.d), 0), 1)
	}
	t = t * t * (3 - 2*t) // ease in and out
	target := 0.0
	if cur.on {
		target = 1
	}
	l := cur.from + (target-cur.from)*t
	if !cur.on && t >= 1 {
		delete(f.fades, id)
	}
	return l
}

// levels are all ids' levels at now, and whether any is still fading.
func (f *fader) levels(now time.Time) (map[string]float64, bool) {
	out := make(map[string]float64, len(f.fades))
	moving := false
	for id, cur := range f.fades {
		if now.Sub(cur.at) < f.d {
			moving = true
		}
		if l := f.level(id, now); l > 0 {
			out[id] = l
		}
	}
	return out, moving
}
