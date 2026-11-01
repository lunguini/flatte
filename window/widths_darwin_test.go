package window

import (
	"strconv"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestWidthCacheGenerations(t *testing.T) {
	c := &widthCache{limit: 2}
	k := func(s string) widthKey { return widthKey{font: 1, text: s} }
	c.put(k("a"), 1)
	c.put(k("b"), 2)
	c.put(k("c"), 3) // turns the generation: a and b are the previous
	if w, ok := c.get(k("a")); !ok || w != 1 {
		t.Fatalf("a from the previous generation: %v %v", w, ok)
	}
	// a was carried into the current generation, which is now full again.
	c.put(k("d"), 4)
	if _, ok := c.get(k("b")); ok {
		t.Fatal("b outlived two turns")
	}
	if w, ok := c.get(k("a")); !ok || w != 1 {
		t.Fatalf("a, in use, was dropped: %v %v", w, ok)
	}
	if _, ok := c.get(widthKey{font: 2, text: "a"}); ok {
		t.Fatal("another font's width")
	}
}

func TestCachedWidthIsShapedWidth(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	for _, k := range []fontKey{{}, {bold: true}, {mono: true}, {size: 30}} {
		f := fontFor(k)
		for _, s := range []string{"", "Overthink", "Kuo, Lin", "♥", "İstanbul 日本"} {
			l := shape(s, f)
			want := l.width
			l.release()
			if got, again := f.width(s), f.width(s); got != want || again != want {
				t.Errorf("%+v %q: width %v then %v, shaped %v", k, s, got, again, want)
			}
		}
	}
}

// BenchmarkSnapshotTable is a table of rich-text rows in Grow columns at
// changing sizes, as a live resize lays it out.
func BenchmarkSnapshotTable(b *testing.B) {
	if err := loadFrameworks(); err != nil {
		b.Fatal(err)
	}
	root := func() layout.Node {
		rows := make([]layout.Node, 30)
		for i := range rows {
			rows[i] = layout.Row{NodeBase: layout.NodeBase{W: layout.Grow(1), Gap: 2}, Children: []layout.Node{
				layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(3)}, Text: strconv.Itoa(i + 1)},
				layout.Label{NodeBase: layout.NodeBase{W: layout.Grow(3)}, Runs: []layout.Run{{Text: "Track title " + strconv.Itoa(i)}, {Text: "  Artist, Another", Style: layout.TextStyle{Role: layout.TextSubtle}}}},
				layout.Label{NodeBase: layout.NodeBase{W: layout.Grow(2)}, Text: "Album name"},
				layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(5)}, Text: "3:20"},
			}}
		}
		return layout.Col{Children: rows}
	}
	app := snapApp(root)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if _, err := Snapshot(app, Options{Width: 90 + i%40, Height: 32}, 1); err != nil {
			b.Fatal(err)
		}
	}
}
