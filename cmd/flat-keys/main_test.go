package main

import (
	"reflect"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestReadSequenceGroupsUntilQuiet(t *testing.T) {
	bytes := make(chan byte, 3)
	bytes <- 0x1b
	bytes <- 0x7f
	bytes <- 'x'

	got, ok := readSequence(bytes, time.Millisecond)
	if !ok {
		t.Fatal("readSequence returned !ok")
	}
	want := []byte{0x1b, 0x7f, 'x'}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readSequence = %#v, want %#v", got, want)
	}
}

func TestDecodeEventsShowsAltBackspaceEncoding(t *testing.T) {
	got := decodeEvents([]byte{0x1b, 0x7f})
	want := []uv.Event{uv.KeyPressEvent{Code: uv.KeyBackspace, Mod: uv.ModAlt}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeEvents = %#v, want %#v", got, want)
	}
}

func TestDecodeEventsShowsEscBSAsCtrlAltH(t *testing.T) {
	got := decodeEvents([]byte{0x1b, 0x08})
	want := []uv.Event{uv.KeyPressEvent{Code: 'h', Mod: uv.ModCtrl | uv.ModAlt}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeEvents = %#v, want %#v", got, want)
	}
}

func TestDescribeSequenceIncludesBytesAndEvents(t *testing.T) {
	got := describeSequence([]byte{0x1b, 0x7f})
	if got == "" {
		t.Fatal("describeSequence returned empty string")
	}
}

func TestAppModesHaveEnterAndExitSequences(t *testing.T) {
	if enterAppModes() == "" {
		t.Fatal("enterAppModes returned empty string")
	}
	if exitAppModes() == "" {
		t.Fatal("exitAppModes returned empty string")
	}
}
