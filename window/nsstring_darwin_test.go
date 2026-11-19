package window

import "testing"

// TestNSStringAcceptsInvalidUTF8 guards the crash a terminal's stray byte
// caused: stringWithUTF8String: answers nil for invalid UTF-8, and drawing
// a nil string throws NSInvalidArgumentException. Invalid bytes become
// U+FFFD instead.
func TestNSStringAcceptsInvalidUTF8(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Skip(err)
	}
	for _, s := range []string{"a\xffb", "\xe2\x9c", "ok ✳", ""} {
		var got string
		withPool(func() {
			id := nsString(s)
			if id == 0 {
				t.Fatalf("nsString(%q) = nil", s)
			}
			got = plainString(id)
		})
		want := map[string]string{"a\xffb": "a�b", "\xe2\x9c": "�", "ok ✳": "ok ✳", "": ""}[s]
		if got != want {
			t.Fatalf("nsString(%q) reads back %q, want %q", s, got, want)
		}
	}
}
