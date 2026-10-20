package window

import (
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func BenchmarkSnapshotFrame(b *testing.B) {
	if err := loadFrameworks(); err != nil {
		b.Fatal(err)
	}
	root := func() layout.Node {
		rows := make([]layout.Node, 30)
		for i := range rows {
			rows[i] = layout.Label{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, Text: "row label with some text"}
		}
		return layout.Col{NodeBase: layout.NodeBase{Pad: 1, Bordered: true}, Children: rows}
	}
	app := snapApp(root)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Snapshot(app, Options{Width: 100, Height: 32}, 2); err != nil {
			b.Fatal(err)
		}
	}
}
