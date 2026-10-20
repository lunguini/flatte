package main

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/brand"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

// IDs of the UI tab's interactive previews. Rows and tabs take these as
// prefixes ("ui.demo.list:2", "ui.demo.tabs:...").
const (
	demoFieldID    = "ui.demo.field"
	demoTextareaID = "ui.demo.textarea"
	demoListID     = "ui.demo.list"
	demoTableID    = "ui.demo.table"
	demoTreeID     = "ui.demo.tree"
	demoTabsID     = "ui.demo.tabs"
	demoViewportID = "ui.demo.viewport"
)

var demoListItems = []string{"inbox", "starred", "sent", "drafts", "archive"}

// demo is the UI tab's live widgets: state, so clicks on a preview act on
// it as they would in an app — select a row or tab, open a tree node, place
// a caret, scroll. editing names the text preview keys go to ("" for none).
type demo struct {
	field   flatui.TextField
	area    flatui.Textarea
	list    flatui.List
	table   flatui.Table
	tree    flatui.Tree
	tabs    *flatui.TabBar
	view    flatui.Viewport
	editing string
}

func newDemo() demo {
	d := demo{
		tree: flatui.NewTree([]flatui.TreeNode{
			{ID: "state", Label: "State", Children: []flatui.TreeNode{{ID: "count", Label: "count"}, {ID: "items", Label: "items"}}},
			{ID: "view", Label: "View"},
		}),
		tabs: flatui.NewTabBar(rightTabs...).WithGlyphs(flatui.TabGlyphsSafe).WithColors(brand.Teal, lipgloss.Color("240"), nil).WithID(demoTabsID),
	}
	d.field.Value = "Ada Lovelace"
	d.field.SetCursor(len(d.field.Value))
	d.area.SetValue("Multiline editing\nwith soft wrap and\nreal cursor cells.")
	d.area.SetSoftWrap(true)
	d.list.SetCount(len(demoListItems))
	d.list.Select(1)
	d.table.SetColumns([]flatui.Column{{Title: "STEP", Width: 6}, {Title: "KIND", Width: 6}, {Title: "STATE", Width: 5}})
	d.table.SetRows([][]string{{"loop", "core", "ok"}, {"view", "pure", "ok"}, {"async", "named", "ok"}})
	d.table.SetHeight(4)
	d.tree.Toggle("state")
	d.tabs.SetActive(1)
	d.view.SetWrappedContent("Viewport keeps body content scrollable while the chrome stays pinned. Wheel and paging are app bindings, not framework policy.")
	return d
}

// size fits the previews to a preview body width×height.
func (d *demo) size(width, height int) {
	width, height = max(width, 1), max(height, 1)
	d.area.SetSize(width, height)
	d.list.SetHeight(min(height, len(demoListItems)))
	d.tree.SetHeight(height)
	d.view.SetSize(width, height)
}

// mouse acts on a preview under the pointer, reporting whether it hit one.
// rects are the page's, for the tab bar's hit test.
func (d *demo) mouse(m flatte.MouseEvent, rects map[string]layout.Rect) bool {
	wheel := 0
	switch m.Button {
	case flatte.MouseWheelUp:
		wheel = -1
	case flatte.MouseWheelDown:
		wheel = 1
	}
	target := m.Target
	switch {
	case wheel != 0:
		return d.scroll(target, wheel)
	case m.Button != flatte.MouseLeft:
		return false
	case m.Action == flatte.MouseMotion:
		// A drag in the text area selects.
		if target == demoTextareaID && d.editing == demoTextareaID && m.HasCaret {
			d.area.MoveToSelecting(m.Caret)
			return true
		}
		return false
	case m.Action != flatte.MousePress:
		return false
	}
	d.editing = ""
	switch {
	case target == demoFieldID && m.HasCaret:
		d.editing = demoFieldID
		d.field.SetCursor(m.Caret)
	case target == demoTextareaID && m.HasCaret:
		d.editing = demoTextareaID
		d.area.MoveTo(m.Caret)
	case strings.HasPrefix(target, demoListID+":"):
		if i, err := strconv.Atoi(strings.TrimPrefix(target, demoListID+":")); err == nil {
			d.list.Select(i)
		}
	case strings.HasPrefix(target, demoTableID+":"):
		if i, err := strconv.Atoi(strings.TrimPrefix(target, demoTableID+":")); err == nil {
			d.table.Select(i)
		}
	case strings.HasPrefix(target, demoTreeID+":"):
		id := strings.TrimPrefix(target, demoTreeID+":")
		if id == d.tree.CursorID() {
			d.tree.Toggle(id)
			break
		}
		for i, row := range d.tree.VisibleRows() {
			if row.ID == id {
				d.tree.SelectVisible(i)
			}
		}
	default:
		if i, ok := d.tabs.HitTest(rects, m.X, m.Y); ok {
			d.tabs.SetActive(i)
			return true
		}
		return target == demoViewportID
	}
	return true
}

// scroll moves the preview under the wheel by one step.
func (d *demo) scroll(target string, by int) bool {
	switch {
	case target == demoViewportID:
		if by > 0 {
			d.view.LineDown(1)
		} else {
			d.view.LineUp(1)
		}
	case strings.HasPrefix(target, demoListID+":"):
		if by > 0 {
			d.list.MoveDown()
		} else {
			d.list.MoveUp()
		}
	case strings.HasPrefix(target, demoTreeID+":"):
		if by > 0 {
			d.tree.MoveDown()
		} else {
			d.tree.MoveUp()
		}
	default:
		return false
	}
	return true
}

// key edits the text preview being edited; Esc stops editing. It reports
// whether the key was taken.
func (d *demo) key(key flatte.KeyEvent) bool {
	if d.editing == "" {
		return false
	}
	if key.Key == flatte.KeyEscape {
		d.editing = ""
		return true
	}
	if d.editing == demoFieldID {
		switch key.Key {
		case flatte.KeyCharacter:
			d.field.Insert(key.Rune)
		case flatte.KeyBackspace:
			d.field.Backspace()
		case flatte.KeyDelete:
			d.field.Delete()
		case flatte.KeyLeft:
			d.field.MoveLeft()
		case flatte.KeyRight:
			d.field.MoveRight()
		default:
			return false
		}
		return true
	}
	selecting := key.Mod.Contains(flatte.ModShift)
	switch key.Key {
	case flatte.KeyCharacter:
		d.area.Insert(key.Rune)
	case flatte.KeyEnter:
		d.area.InsertNewline()
	case flatte.KeyBackspace:
		d.area.Backspace()
	case flatte.KeyDelete:
		d.area.Delete()
	case flatte.KeyLeft:
		if selecting {
			d.area.MoveLeftSelecting()
		} else {
			d.area.MoveLeft()
		}
	case flatte.KeyRight:
		if selecting {
			d.area.MoveRightSelecting()
		} else {
			d.area.MoveRight()
		}
	case flatte.KeyUp, flatte.KeyDown:
		switch {
		case key.HasCaret && selecting:
			d.area.MoveToSelecting(key.Caret)
		case key.HasCaret:
			d.area.MoveTo(key.Caret)
		case key.Key == flatte.KeyUp:
			d.area.MoveUp()
		default:
			d.area.MoveDown()
		}
	default:
		return false
	}
	return true
}

// paste inserts pasted text into the preview being edited.
func (d *demo) paste(text string) {
	switch d.editing {
	case demoFieldID:
		for _, r := range text {
			if r != '\n' && r != '\r' {
				d.field.Insert(r)
			}
		}
	case demoTextareaID:
		d.area.InsertText(text)
	}
}
