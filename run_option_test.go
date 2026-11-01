package flatte

import (
	"context"
	"testing"
)

type optionHost struct{ name string }

func (h optionHost) Run(context.Context, *Session) error { return nil }

type installHostOption struct{ host Host }

func (o installHostOption) ConfigureRun(c *RunConfig) { c.UseHost(o.host) }

type fallbackHostOption struct{ host Host }

func (o fallbackHostOption) ConfigureRun(c *RunConfig) { c.UseFallbackHost(o.host) }

func TestFallbackHostSelection(t *testing.T) {
	window := optionHost{name: "window"}
	media := optionHost{name: "media"}
	tests := []struct {
		name    string
		options []Option
		want    Host
	}{
		{name: "fallback only", options: []Option{fallbackHostOption{media}}, want: media},
		{name: "ordinary host only", options: []Option{installHostOption{window}}, want: window},
		{name: "ordinary host before fallback", options: []Option{installHostOption{window}, fallbackHostOption{media}}, want: window},
		{name: "fallback before ordinary host", options: []Option{fallbackHostOption{media}, installHostOption{window}}, want: window},
		{name: "duplicate fallback", options: []Option{fallbackHostOption{media}, fallbackHostOption{media}}, want: media},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := applyRunOptions(runConfig{}, test.options...)
			if got.err != nil {
				t.Fatalf("option error = %v", got.err)
			}
			if selected := got.selectedHost(); selected != test.want {
				t.Fatalf("selected host = %#v, want %#v", selected, test.want)
			}
		})
	}
}

func TestConflictingOrdinaryHostsStillFail(t *testing.T) {
	got := applyRunOptions(runConfig{},
		installHostOption{optionHost{name: "window"}},
		installHostOption{optionHost{name: "other"}},
	)
	if got.err != ErrMultipleHosts {
		t.Fatalf("option error = %v, want ErrMultipleHosts", got.err)
	}
}
