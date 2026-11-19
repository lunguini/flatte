package flatte

import "testing"

func TestAppearanceDarkTreatsUnknownAsDark(t *testing.T) {
	for a, want := range map[Appearance]bool{AppearanceUnknown: true, AppearanceDark: true, AppearanceLight: false} {
		if got := a.Dark(); got != want {
			t.Errorf("%d.Dark() = %v, want %v", a, got, want)
		}
	}
}
