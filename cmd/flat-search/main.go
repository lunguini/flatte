package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
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
	switch e := ev.(type) {
	case flatte.PasteEvent:
		if s.focused {
			before := s.query.Value
			s.query.InsertText(e.Text)
			if s.query.Value != before {
				startSearch(s, fx)
			}
		}
		return
	case flatte.KeyEvent:
		handleKey(s, e, fx)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
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

	if key.Key == flatte.KeyEnter {
		s.focused = false
		return
	}
	// Keep the terminal's Alt+B/F word-movement convention; native word
	// movement and all other editing are handled by TextField.Edit.
	if key.Key == flatte.KeyCharacter && handleAltWordKey(key, s.query.MoveWordLeft, s.query.MoveWordRight) {
		return
	}
	before := s.query.Value
	if s.query.Edit(key) && s.query.Value != before {
		startSearch(s, fx)
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

var (
	title  = layout.TextStyle{Role: layout.TextTitle}
	subtle = layout.TextStyle{Role: layout.TextSubtle}
	indent = layout.NodeBase{PadLeft: 2}
)

// View shows the query input, status, and results. The query Input places
// the cursor while focused, so there is no cursor arithmetic here.
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	status := "idle"
	if s.searching {
		status = "searching..."
	}
	if s.err != nil {
		status = s.err.Error()
	}
	children := []layout.Node{
		layout.Label{Text: "Flat Search", Style: title},
		layout.Label{Text: "input-triggered async sample", Style: subtle},
		layout.Label{},
		layout.Row{NodeBase: indent, Children: []layout.Node{
			layout.Label{Text: "query: "},
			s.query.Layout("search.query", s.focused),
		}},
		layout.Label{NodeBase: indent, Text: "state: " + status},
		layout.Label{},
	}
	if len(s.results) == 0 {
		children = append(children, layout.Label{NodeBase: indent, Text: "no results", Style: subtle})
	} else {
		for _, result := range s.results {
			children = append(children, layout.Label{NodeBase: indent, Text: "- " + result})
		}
	}
	children = append(children, layout.Label{}, layout.Label{Text: "enter blur/focus | q quits when blurred", Style: subtle})
	return flatte.Frame{Root: flatui.CardLayout(children...)}
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
