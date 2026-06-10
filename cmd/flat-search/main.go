package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
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

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	if !s.focused {
		if ev.Key == flatcore.KeyCharacter && (ev.Rune == 'q' || ev.Rune == 'Q') {
			fx.Quit()
		}
		if ev.Key == flatcore.KeyEnter {
			s.focused = true
			s.query.SetCursor(len(s.query.Value))
		}
		return
	}

	switch ev.Key {
	case flatcore.KeyCharacter:
		s.query.Insert(ev.Rune)
		startSearch(s, fx)
	case flatcore.KeyBackspace:
		s.query.Backspace()
		startSearch(s, fx)
	case flatcore.KeyDelete:
		s.query.Delete()
		startSearch(s, fx)
	case flatcore.KeyLeft:
		s.query.MoveLeft()
	case flatcore.KeyRight:
		s.query.MoveRight()
	case flatcore.KeyEnter:
		s.focused = false
	}
}

func startSearch(s *State, fx flatcore.Effects[State]) {
	query := s.query.Value
	s.err = nil

	if strings.TrimSpace(query) == "" {
		s.searching = false
		s.results = nil
		flatcore.Cancel(fx, "search.run")
		return
	}

	s.searching = true
	flatcore.Latest(fx, "search.run",
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

func View(s *State, ctx flatcore.RenderContext) string {
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
	return flatui.Card(rows, ctx.Width)
}

func renderQuery(s *State) string {
	return s.query.Render(s.focused)
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
	err := flatcore.Run(context.Background(), flatcore.App[State]{
		State:  state,
		Handle: Handle,
		View:   View,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
