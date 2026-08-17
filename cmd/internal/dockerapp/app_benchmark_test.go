package dockerapp

import (
	"testing"

	"github.com/lunguini/flatte"
)

// BenchmarkIgnoredMouseMotion guards the ordinary motion path: an event that
// does not change geometry should not solve the three-screen frame tree.
func BenchmarkIgnoredMouseMotion(b *testing.B) {
	s := resizedState(120, 40)
	ev := flatte.MouseEvent{Action: flatte.MouseMotion, Button: flatte.MouseNone, X: 4, Y: 4}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Handle(s, ev, flatte.Effects[State]{})
	}
}
