package flatui

import "unicode/utf8"

type TextField struct {
	Value  string
	Cursor int
}

func (f *TextField) Insert(r rune) {
	f.clampCursor()
	text := string(r)
	f.Value = f.Value[:f.Cursor] + text + f.Value[f.Cursor:]
	f.Cursor += len(text)
}

func (f *TextField) Backspace() {
	f.clampCursor()
	if f.Cursor == 0 {
		return
	}
	_, size := utf8.DecodeLastRuneInString(f.Value[:f.Cursor])
	f.Value = f.Value[:f.Cursor-size] + f.Value[f.Cursor:]
	f.Cursor -= size
}

func (f *TextField) Delete() {
	f.clampCursor()
	if f.Cursor >= len(f.Value) {
		return
	}
	_, size := utf8.DecodeRuneInString(f.Value[f.Cursor:])
	f.Value = f.Value[:f.Cursor] + f.Value[f.Cursor+size:]
}

func (f *TextField) MoveLeft() {
	f.clampCursor()
	if f.Cursor == 0 {
		return
	}
	_, size := utf8.DecodeLastRuneInString(f.Value[:f.Cursor])
	f.Cursor -= size
}

func (f *TextField) MoveRight() {
	f.clampCursor()
	if f.Cursor >= len(f.Value) {
		return
	}
	_, size := utf8.DecodeRuneInString(f.Value[f.Cursor:])
	f.Cursor += size
}

func (f *TextField) SetCursor(cursor int) {
	f.Cursor = cursor
	f.clampCursor()
}

func (f TextField) Render(focused bool) string {
	f.clampCursor()
	if !focused {
		return f.Value
	}
	return f.Value[:f.Cursor] + "▌" + f.Value[f.Cursor:]
}

func (f *TextField) clampCursor() {
	if f.Cursor < 0 {
		f.Cursor = 0
	}
	if f.Cursor > len(f.Value) {
		f.Cursor = len(f.Value)
	}
	for f.Cursor > 0 && f.Cursor < len(f.Value) && !utf8.RuneStart(f.Value[f.Cursor]) {
		f.Cursor--
	}
}
