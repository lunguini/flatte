package main

import (
	"math"
	"unicode/utf16"
	"unicode/utf8"
)

// span is a text range in UTF-16 code units, the unit NSTextInputClient
// speaks. Missing mirrors an NSRange whose location is NSNotFound.
type span struct {
	Loc, Len int
	Missing  bool
}

var noSpan = span{Missing: true}

// editBuffer is the probe's single-line text model: committed text followed
// by an optional input-method composition ("marked text"). The composition
// always sits at the end; reconverting text in the middle is out of the
// probe's scope.
type editBuffer struct {
	committed string
	marked    string
	markedSel span // selection inside marked, relative to its start
}

// Text returns the committed text followed by the composition.
func (b *editBuffer) Text() string { return b.committed + b.marked }

func (b *editBuffer) HasMarked() bool { return b.marked != "" }

func (b *editBuffer) MarkedRange() span {
	if b.marked == "" {
		return noSpan
	}
	return span{Loc: utf16Len(b.committed), Len: utf16Len(b.marked)}
}

// SelectedRange is the caret, or the selection inside the composition.
func (b *editBuffer) SelectedRange() span {
	base := utf16Len(b.committed)
	if b.marked == "" {
		return span{Loc: base}
	}
	return span{Loc: base + b.markedSel.Loc, Len: b.markedSel.Len}
}

// Insert commits s. Without a replacement range it replaces the composition,
// or appends when there is none; a present range is replaced within Text.
// Either way the composition ends.
func (b *editBuffer) Insert(s string, replace span) {
	if replace.Missing {
		b.committed += s
	} else {
		b.committed = replaceSpan(b.Text(), replace, s)
	}
	b.marked, b.markedSel = "", span{}
}

// SetMarked replaces the composition with s; an empty s cancels it. sel is
// the selection within s. A present replacement range is removed from Text
// first, so the new composition takes its place at the end.
func (b *editBuffer) SetMarked(s string, sel, replace span) {
	if !replace.Missing {
		b.committed = replaceSpan(b.Text(), replace, "")
	}
	b.marked = s
	n := utf16Len(s)
	if sel.Missing {
		b.markedSel = span{Loc: n}
		return
	}
	loc := clamp(sel.Loc, 0, n)
	b.markedSel = span{Loc: loc, Len: clamp(sel.Len, 0, n-loc)}
}

// Unmark commits the composition as it stands.
func (b *editBuffer) Unmark() {
	b.committed += b.marked
	b.marked, b.markedSel = "", span{}
}

// DeleteBackward removes the last rune of committed text. The input method
// owns an active composition, so it is left alone. Grapheme-correct editing
// is flatui.TextField's job once a real host exists.
func (b *editBuffer) DeleteBackward() {
	if b.marked != "" {
		return
	}
	_, n := utf8.DecodeLastRuneInString(b.committed)
	b.committed = b.committed[:len(b.committed)-n]
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// byteOffset converts a UTF-16 offset into a byte offset in s, clamped to
// [0, len(s)]. An offset inside a surrogate pair rounds up past the rune.
func byteOffset(s string, u16 int) int {
	if u16 <= 0 {
		return 0
	}
	n := 0
	for i, r := range s {
		if n >= u16 {
			return i
		}
		n += utf16.RuneLen(r)
	}
	return len(s)
}

// replaceSpan replaces r (clamped to s) with with.
func replaceSpan(s string, r span, with string) string {
	loc, n := max(r.Loc, 0), max(r.Len, 0)
	start, end := byteOffset(s, loc), len(s)
	if n < math.MaxInt-loc {
		end = byteOffset(s, loc+n)
	}
	return s[:start] + with + s[end:]
}

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }
