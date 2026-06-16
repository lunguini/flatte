package flatui

import "testing"

func TestTextFieldInsertBackspaceDeleteAndCursorMovement(t *testing.T) {
	field := TextField{Value: "abcd", Cursor: 2}

	field.MoveLeft()
	field.Backspace()
	field.Delete()
	field.Insert('Z')
	field.MoveRight()

	if field.Value != "Zcd" {
		t.Fatalf("Value = %q, want Zcd", field.Value)
	}
	if field.Cursor != 2 {
		t.Fatalf("Cursor = %d, want 2", field.Cursor)
	}
}

func TestTextFieldHandlesMultibyteRunes(t *testing.T) {
	field := TextField{}

	field.Insert('ă')
	field.Insert('b')
	field.MoveLeft()
	field.Backspace()

	if field.Value != "b" {
		t.Fatalf("Value = %q, want b", field.Value)
	}
	if field.Cursor != 0 {
		t.Fatalf("Cursor = %d, want 0", field.Cursor)
	}
}

func TestTextFieldSetCursorClampsToRuneBoundary(t *testing.T) {
	field := TextField{Value: "aăb"}

	field.SetCursor(2) // inside the 2-byte ă: must clamp back to its start

	if field.Cursor != 1 {
		t.Fatalf("Cursor = %d, want clamped to byte 1", field.Cursor)
	}
	if got := field.CursorColumn(); got != 1 {
		t.Fatalf("CursorColumn() = %d, want 1", got)
	}
}

func TestTextFieldMovesAndDeletesByGraphemeCluster(t *testing.T) {
	// "a" + combining acute (U+0301) forms ONE grapheme cluster (two runes,
	// three bytes), then "bc". Movement and deletion must treat the cluster as
	// an indivisible unit, not step into it rune-by-rune. The ́ escape is
	// deliberate — a literal "á" would be the precomposed single rune.
	base := "ábc" // grapheme boundaries [0,3,4,5]

	f := TextField{Value: base}
	f.MoveRight() // skip the whole cluster, not just the base 'a'
	if f.Cursor != 3 {
		t.Fatalf("MoveRight cursor = %d, want 3 (past the combining cluster)", f.Cursor)
	}
	f.MoveLeft()
	if f.Cursor != 0 {
		t.Fatalf("MoveLeft cursor = %d, want 0", f.Cursor)
	}

	bs := TextField{Value: base, Cursor: 3}
	bs.Backspace() // removes the whole cluster
	if bs.Value != "bc" || bs.Cursor != 0 {
		t.Fatalf("Backspace -> %q cursor %d, want \"bc\" 0", bs.Value, bs.Cursor)
	}

	del := TextField{Value: base, Cursor: 0}
	del.Delete() // removes the whole cluster
	if del.Value != "bc" {
		t.Fatalf("Delete -> %q, want \"bc\"", del.Value)
	}
}

func TestTextFieldMovesByWord(t *testing.T) {
	value := "hello, world café"
	field := TextField{Value: value, Cursor: len(value)}

	field.MoveWordLeft()
	if field.Cursor != len("hello, world ") {
		t.Fatalf("MoveWordLeft from end = %d, want start of café", field.Cursor)
	}
	field.MoveWordLeft()
	if field.Cursor != len("hello, ") {
		t.Fatalf("second MoveWordLeft = %d, want start of world", field.Cursor)
	}
	field.MoveWordLeft()
	if field.Cursor != 0 {
		t.Fatalf("third MoveWordLeft = %d, want start", field.Cursor)
	}

	field.MoveWordRight()
	if field.Cursor != len("hello") {
		t.Fatalf("MoveWordRight from start = %d, want end of hello", field.Cursor)
	}
	field.MoveWordRight()
	if field.Cursor != len("hello, world") {
		t.Fatalf("second MoveWordRight = %d, want end of world", field.Cursor)
	}
	field.MoveWordRight()
	if field.Cursor != len(value) {
		t.Fatalf("third MoveWordRight = %d, want end", field.Cursor)
	}
}

func TestTextFieldDeletesByWord(t *testing.T) {
	field := TextField{Value: "hello, world café", Cursor: len("hello, world")}

	field.DeleteWordLeft()
	if field.Value != "hello,  café" {
		t.Fatalf("DeleteWordLeft value = %q, want %q", field.Value, "hello,  café")
	}
	if field.Cursor != len("hello, ") {
		t.Fatalf("DeleteWordLeft cursor = %d, want %d", field.Cursor, len("hello, "))
	}

	field.DeleteWordRight()
	if field.Value != "hello, " {
		t.Fatalf("DeleteWordRight value = %q, want %q", field.Value, "hello, ")
	}
	if field.Cursor != len("hello, ") {
		t.Fatalf("DeleteWordRight cursor = %d, want %d", field.Cursor, len("hello, "))
	}
}

func TestCursorColumnCountsDisplayCells(t *testing.T) {
	cases := []struct {
		name   string
		value  string
		cursor int // byte offset
		want   int
	}{
		{"empty", "", 0, 0},
		{"ascii middle", "hello", 3, 3},
		{"after multibyte rune", "héllo", 3, 2},
		{"after wide rune", "a世b", 4, 3},
		{"clamped past end", "hi", 99, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			field := TextField{Value: tc.value, Cursor: tc.cursor}
			if got := field.CursorColumn(); got != tc.want {
				t.Fatalf("CursorColumn() = %d, want %d", got, tc.want)
			}
		})
	}
}
