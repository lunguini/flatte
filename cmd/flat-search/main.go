package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
)

const defaultSearchDelay = 300 * time.Millisecond

var corpus = []string{
	"haiku",
	"sonnet",
	"opus",
	"freeform",
	"villanelle",
	"limerick",
	"ghazal",
}

type State struct {
	query     flatui.TextField
	focused   bool
	searching bool
	results   []string
	err       error
}

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	key, ok := ev.(flatte.KeyEvent)
	if !ok {
		return
	}
	if !s.focused {
		if key.Key == flatte.KeyCharacter && (key.Rune == 'q' || key.Rune == 'Q') {
			fx.Quit()
		}
		if key.Key == flatte.KeyEnter {
			s.focused = true
			s.query.SetCursor(len(s.query.Value))
		}
		return
	}

	switch key.Key {
	case flatte.KeyCharacter:
		if handleAltWordKey(key, s.query.MoveWordLeft, s.query.MoveWordRight) {
			return
		}
		s.query.Insert(key.Rune)
		startSearch(s, fx)
	case flatte.KeyBackspace:
		s.query.Backspace()
		startSearch(s, fx)
	case flatte.KeyDelete:
		s.query.Delete()
		startSearch(s, fx)
	case flatte.KeyLeft:
		s.query.MoveLeft()
	case flatte.KeyRight:
		s.query.MoveRight()
	case flatte.KeyEnter:
		s.focused = false
	}
}

func handleAltWordKey(key flatte.KeyEvent, moveLeft, moveRight func()) bool {
	if !key.Mod.Contains(flatte.ModAlt) {
		return false
	}
	switch key.Rune {
	case 'b', 'B':
		moveLeft()
		return true
	case 'f', 'F':
		moveRight()
		return true
	}
	return false
}

func startSearch(s *State, fx flatte.Effects[State]) {
	query := s.query.Value
	s.err = nil

	if strings.TrimSpace(query) == "" {
		s.searching = false
		s.results = nil
		flatte.Cancel(fx, "search.run")
		return
	}

	s.searching = true
	flatte.Latest(fx, "search.run",
		func(ctx context.Context) ([]string, error) {
			return search(ctx, query)
		},
		func(s *State, results []string, err error) {
			s.searching = false
			if err != nil {
				s.err = err
				return
			}
			s.results = results
		},
	)
}

func search(ctx context.Context, query string) ([]string, error) {
	select {
	case <-time.After(searchDelay()):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	query = strings.ToLower(query)
	var results []string
	for _, item := range corpus {
		if strings.Contains(strings.ToLower(item), query) {
			results = append(results, item)
		}
	}
	return results, nil
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	status := "idle"
	if s.searching {
		status = "searching..."
	}
	if s.err != nil {
		status = s.err.Error()
	}

	rows := []string{
		flatui.Title("Flat Search"),
		flatui.Subtle("input-triggered async sample"),
		"",
		"  query: " + renderQuery(s),
		"  state: " + status,
		"",
	}
	if len(s.results) == 0 {
		rows = append(rows, flatui.Subtle("  no results"))
	} else {
		for _, result := range s.results {
			rows = append(rows, "  - "+result)
		}
	}
	rows = append(rows, "", flatui.Subtle("enter blur/focus | q quits when blurred"))
	frame := flatte.Frame{Content: flatui.Card(rows, ctx.Width)}
	if s.focused {
		originX, originY := flatui.CardOrigin()
		frame.Cursor = &flatte.Cursor{
			X: originX + lipgloss.Width("  query: ") + s.query.CursorColumn(),
			Y: originY + 3, // title, subtle, blank precede the query row
		}
	}
	return frame
}

func renderQuery(s *State) string {
	return s.query.Value
}

func searchDelay() time.Duration {
	value := os.Getenv("FLAT_SEARCH_DELAY")
	if value == "" {
		return defaultSearchDelay
	}
	delay, err := time.ParseDuration(value)
	if err != nil || delay < 0 {
		return defaultSearchDelay
	}
	return delay
}

func main() {
	state := &State{focused: true}
	err := flatte.Run(context.Background(), flatte.App[State]{
		State:  state,
		Handle: Handle,
		View:   View,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
