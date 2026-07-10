package snakeapp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/snakesim"
	"github.com/lunguini/flatte/flatest"
)

func stripAnsi(s string) string { return ansi.Strip(s) }

const move1 = time.Duration(startMoveMs) * time.Millisecond // one level-1 move

func keyChar(r rune) flatte.KeyEvent { return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r} }

func driver(seed uint64) *flatest.Driver[State] {
	return flatest.Start(flatte.App[State]{
		State:  newGame(seed),
		Init:   Init,
		Handle: Handle,
		View:   View,
	}, frameW)
}

func headOf(d *flatest.Driver[State]) snakesim.Point { return d.State().game.Snake[0] }

// --- Movement ---

func TestSnakeMovesRightOnTick(t *testing.T) {
	d := driver(1)
	start := headOf(d)
	d.Advance(move1)
	if got := headOf(d); got != (snakesim.Point{X: start.X + 1, Y: start.Y}) {
		t.Fatalf("after one move: head = %v, want %v", got, snakesim.Point{X: start.X + 1, Y: start.Y})
	}
	if l := len(d.State().game.Snake); l != 3 {
		t.Fatalf("length changed without food: %d, want 3", l)
	}
}

func TestNoMoveBeforeAFullInterval(t *testing.T) {
	d := driver(1)
	start := headOf(d)
	d.Advance(move1 - baseInterval) // one base tick short of a step
	if got := headOf(d); got != start {
		t.Fatalf("moved before a full interval elapsed: %v -> %v", start, got)
	}
}

// --- Steering ---

func TestSteeringAppliesOnNextTick(t *testing.T) {
	d := driver(2)
	start := headOf(d)
	d.Send(keyChar('s')) // down
	d.Advance(move1)
	if got := headOf(d); got != (snakesim.Point{X: start.X, Y: start.Y + 1}) {
		t.Fatalf("after steer down + tick: head = %v, want %v", got, snakesim.Point{X: start.X, Y: start.Y + 1})
	}
}

func TestReversingIntoSelfIsIgnored(t *testing.T) {
	d := driver(2)
	start := headOf(d)
	d.Send(keyChar('a')) // left is the reverse of the initial right heading
	d.Advance(move1)
	if got := headOf(d); got != (snakesim.Point{X: start.X + 1, Y: start.Y}) {
		t.Fatalf("reversal should be ignored; head = %v, want %v", got, snakesim.Point{X: start.X + 1, Y: start.Y})
	}
}

func TestFastDoubleTurnQueuesBothTurns(t *testing.T) {
	// Moving right, press Up then Left within one tick. Left is the opposite
	// of the CURRENT heading but perfectly legal after the queued Up — it must
	// queue behind it (validated against the effective heading), not be
	// dropped as a reversal. One turn applies per tick: up, then left.
	d := driver(2)
	start := headOf(d)
	d.Send(flatte.KeyEvent{Key: flatte.KeyUp})
	d.Send(flatte.KeyEvent{Key: flatte.KeyLeft})
	d.Advance(move1)
	if got := headOf(d); got != (snakesim.Point{X: start.X, Y: start.Y - 1}) {
		t.Fatalf("first queued turn (up) should apply; head = %v, want %v", got, snakesim.Point{X: start.X, Y: start.Y - 1})
	}
	d.Advance(move1)
	if got := headOf(d); got != (snakesim.Point{X: start.X - 1, Y: start.Y - 1}) {
		t.Fatalf("second queued turn (left) should apply next tick; head = %v, want %v",
			got, snakesim.Point{X: start.X - 1, Y: start.Y - 1})
	}
}

func TestQueuedReversalIsStillIgnored(t *testing.T) {
	// Up then Down within one tick: Down reverses the queued Up and is
	// dropped — the snake turns up and stays up.
	d := driver(2)
	start := headOf(d)
	d.Send(flatte.KeyEvent{Key: flatte.KeyUp})
	d.Send(flatte.KeyEvent{Key: flatte.KeyDown})
	d.Advance(move1)
	if got := headOf(d); got != (snakesim.Point{X: start.X, Y: start.Y - 1}) {
		t.Fatalf("queued turn (up) should apply; head = %v, want %v", got, snakesim.Point{X: start.X, Y: start.Y - 1})
	}
	d.Advance(move1)
	if got := headOf(d); got != (snakesim.Point{X: start.X, Y: start.Y - 2}) {
		t.Fatalf("reversal of the queued turn must be dropped; head = %v, want %v",
			got, snakesim.Point{X: start.X, Y: start.Y - 2})
	}
}

// --- Growth & food ---

// feedAhead places food directly in front of the head and steps into it.
func feedAhead(s *State) {
	v := s.game.Dir.Vec()
	s.game.Food = snakesim.Point{X: s.game.Snake[0].X + v.X, Y: s.game.Snake[0].Y + v.Y}
	s.game.Step()
}

func TestEatingFoodGrowsAndScores(t *testing.T) {
	s := newGame(7)
	before := len(s.game.Snake)
	feedAhead(s)
	if got := len(s.game.Snake); got != before+1 {
		t.Fatalf("length after eating = %d, want %d", got, before+1)
	}
	if s.game.FoodEaten != 1 {
		t.Fatalf("foodEaten = %d, want 1", s.game.FoodEaten)
	}
	if s.game.Score != s.game.Level {
		t.Fatalf("score = %d, want level %d", s.game.Score, s.game.Level)
	}
	if s.game.Over {
		t.Fatal("eating food should not end the game")
	}
	for _, p := range s.game.Snake {
		if p == s.game.Food {
			t.Fatalf("respawned food %v lands on the snake", s.game.Food)
		}
	}
}

func TestFiveFoodRaisesLevelAndSpeed(t *testing.T) {
	s := newGame(9)
	for i := 0; i < snakesim.FoodPerLevel; i++ {
		feedAhead(s)
	}
	if s.game.Level != 2 {
		t.Fatalf("after %d food: level = %d, want 2", snakesim.FoodPerLevel, s.game.Level)
	}
	if got := s.moveIntervalMs(); got != startMoveMs-stepMoveMs {
		t.Fatalf("level 2 move interval = %dms, want %dms", got, startMoveMs-stepMoveMs)
	}
	if s.game.Score != 6 { // four foods at level 1 + one at level 2
		t.Fatalf("score after 5 food = %d, want 6", s.game.Score)
	}
}

func TestHighScoreUpdatesWhenBeaten(t *testing.T) {
	d := driver(4)
	d.State().HighScore = 2
	// Drive three foods through the ticker so onTick updates HighScore.
	for i := 0; i < 3; i++ {
		s := d.State()
		v := s.game.Dir.Vec()
		s.game.Food = snakesim.Point{X: s.game.Snake[0].X + v.X, Y: s.game.Snake[0].Y + v.Y}
		d.Advance(move1)
	}
	if d.State().game.Score != 3 {
		t.Fatalf("score = %d, want 3", d.State().game.Score)
	}
	if d.State().HighScore != 3 {
		t.Fatalf("high score = %d, want it raised to 3", d.State().HighScore)
	}
}

// --- Collision ---

func TestWallCollisionEndsGame(t *testing.T) {
	s := newGame(1)
	s.game.Snake = []snakesim.Point{{X: snakesim.GridW - 1, Y: 5}, {X: snakesim.GridW - 2, Y: 5}, {X: snakesim.GridW - 3, Y: 5}}
	s.game.Dir = snakesim.Right
	s.game.Step()
	if !s.game.Over {
		t.Fatal("moving into the right wall should end the game")
	}
}

func TestSelfCollisionEndsGame(t *testing.T) {
	s := newGame(1)
	// A square loop; turning down from the head runs into a non-tail body cell.
	s.game.Snake = []snakesim.Point{{X: 5, Y: 5}, {X: 5, Y: 6}, {X: 6, Y: 6}, {X: 6, Y: 5}}
	s.game.Dir, s.game.DirQueue = snakesim.Right, []snakesim.Direction{snakesim.Down}
	s.game.Food = snakesim.Point{X: 0, Y: 0} // not adjacent, so this is not an eat
	s.game.Step()
	if !s.game.Over {
		t.Fatal("moving into the snake's own body should end the game")
	}
}

func TestMovingIntoVacatingTailIsSafe(t *testing.T) {
	s := newGame(1)
	// Head about to enter the tail cell, which vacates this step — legal.
	s.game.Snake = []snakesim.Point{{X: 5, Y: 5}, {X: 5, Y: 6}, {X: 6, Y: 6}, {X: 6, Y: 5}}
	s.game.Dir, s.game.DirQueue = snakesim.Up, []snakesim.Direction{snakesim.Right} // {5,5} -> {6,5} (the tail)
	s.game.Food = snakesim.Point{X: 0, Y: 0}
	s.game.Step()
	if s.game.Over {
		t.Fatal("moving into the vacating tail cell should be safe")
	}
}

// --- Speed (effective, via the harness clock) ---

func TestHigherLevelMovesFasterPerWindow(t *testing.T) {
	d := driver(3)
	start := headOf(d)
	// A 200ms window is short of one level-1 (220ms) move.
	d.Advance(time.Duration(startMoveMs-stepMoveMs) * time.Millisecond)
	if headOf(d) != start {
		t.Fatalf("level 1 should not complete a move in %dms", startMoveMs-stepMoveMs)
	}
	// At level 2 the same window is exactly one move.
	d.State().game.Level = 2
	d.State().stepAccum = 0
	d.Advance(time.Duration(startMoveMs-stepMoveMs) * time.Millisecond)
	if got := headOf(d); got != (snakesim.Point{X: start.X + 1, Y: start.Y}) {
		t.Fatalf("level 2 should complete one move in %dms; head = %v", startMoveMs-stepMoveMs, got)
	}
}

// --- Pause (Scope.Cancel stops the ticker) ---

func TestPauseStopsTicksThenResumeContinues(t *testing.T) {
	d := driver(5)
	start := headOf(d)
	d.Send(keyChar('p')) // pause -> ticker cancelled
	if !d.State().paused {
		t.Fatal("p should pause")
	}
	d.Advance(10 * move1)
	if headOf(d) != start {
		t.Fatalf("a paused game must not move: %v -> %v", start, headOf(d))
	}
	d.Send(keyChar('p')) // resume -> fresh ticker
	if d.State().paused {
		t.Fatal("second p should resume")
	}
	d.Advance(move1)
	if got := headOf(d); got != (snakesim.Point{X: start.X + 1, Y: start.Y}) {
		t.Fatalf("resumed game should move: head = %v, want %v", got, snakesim.Point{X: start.X + 1, Y: start.Y})
	}
}

// --- Restart ---

func TestRestartResetsButKeepsHighScore(t *testing.T) {
	d := driver(11)
	// Drive into the top wall to end the game deterministically.
	d.Send(flatte.KeyEvent{Key: flatte.KeyUp})
	d.Advance(40 * move1)
	if !d.State().game.Over {
		t.Fatalf("expected game over after steering into the top wall; head=%v", headOf(d))
	}
	d.State().HighScore = 99

	d.Send(keyChar('r'))
	st := d.State()
	if st.game.Over || st.paused {
		t.Fatal("restart should clear over/paused")
	}
	if st.game.Score != 0 || st.game.Level != 1 || len(st.game.Snake) != 3 {
		t.Fatalf("restart board not reset: score=%d level=%d len=%d", st.game.Score, st.game.Level, len(st.game.Snake))
	}
	if st.HighScore != 99 {
		t.Fatalf("restart lost the high score: %d, want 99", st.HighScore)
	}
	if st.game.Snake[0] != (snakesim.Point{X: snakesim.GridW / 2, Y: snakesim.GridH / 2}) {
		t.Fatalf("restart head = %v, want board center", st.game.Snake[0])
	}
	// The ticker was re-armed, so play continues.
	d.Advance(move1)
	if st.game.Snake[0] == (snakesim.Point{X: snakesim.GridW / 2, Y: snakesim.GridH / 2}) {
		t.Fatal("restarted game did not resume ticking")
	}
}

// --- Quit ---

func TestQuitKeys(t *testing.T) {
	for _, key := range []flatte.KeyEvent{keyChar('q'), {Key: flatte.KeyEscape}} {
		var quit bool
		fx := flatte.NewEffects[State](context.Background(), nil, func() { quit = true })
		Handle(newGame(1), key, fx)
		if !quit {
			t.Fatalf("%+v did not request quit", key)
		}
	}
}

func TestGameOverIgnoresSteeringButAcceptsRestart(t *testing.T) {
	s := newGame(1)
	s.game.Over = true
	fx := flatte.NewEffects[State](context.Background(), nil, func() {})
	Handle(s, keyChar('w'), fx) // steering ignored while over
	if len(s.game.DirQueue) != 0 {
		t.Fatal("steering should be ignored on the game-over screen")
	}
}

// --- Persisted state ---

func TestSaveStatePersistsOnlyHighScore(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/state.gob"
	// A live game with a fully populated board. Only HighScore is exported, so
	// gob must persist that and drop the unexported *snakesim.Game entirely.
	s := newGame(1)
	s.game.Score = 7
	s.game.Level = 5
	s.HighScore = 42
	if err := flatte.SaveState(path, *s); err != nil {
		t.Fatal(err)
	}
	got := flatte.LoadState(path, State{})
	if got.HighScore != 42 {
		t.Fatalf("loaded HighScore = %d, want 42", got.HighScore)
	}
	if got.game != nil {
		t.Fatalf("unexported game field should not persist, got %+v", got.game)
	}
}

// --- Goldens ---

// goldenFrames builds the deterministic frames pinned as goldens. gen_goldens
// writes them; the Test* below assert against them.
func goldenFrames() []struct {
	name  string
	frame flatte.Frame
} {
	rc := flatte.RenderContext{Width: frameW}

	initial := newGame(42)

	moved := newGame(42)
	for i := 0; i < 3; i++ {
		moved.game.Step() // three cells right (no food eaten)
	}
	moved.game.Steer(snakesim.Down)
	moved.game.Step()
	moved.game.Step() // then two cells down: an L-shaped body

	over := newGame(42)
	over.game.Over = true
	over.game.Score = 6
	over.game.Level = 2
	over.HighScore = 10

	paused := newGame(42)
	paused.paused = true

	return []struct {
		name  string
		frame flatte.Frame
	}{
		{"initial", View(initial, rc)},
		{"moved", View(moved, rc)},
		{"gameover", View(over, rc)},
		{"pause", View(paused, rc)},
	}
}

func TestGoldenFrames(t *testing.T) {
	for _, c := range goldenFrames() {
		t.Run(c.name, func(t *testing.T) {
			flatest.AssertGoldenFrame(t, "testdata/"+c.name+".golden", c.frame)
		})
	}
}

// The board pane's border is the collision wall: on a terminal taller than the
// grid, the bottom border must sit directly under the last grid row, not
// stretch to the terminal edge (a stretched border makes the play area look
// bigger than it is).
func TestBoardBorderMatchesPlayArea(t *testing.T) {
	s := newGame(42)
	s.width, s.height = frameW, 40 // taller than the 24-row game frame

	content := stripAnsi(View(s, flatte.RenderContext{Width: frameW}).Content)
	lines := strings.Split(content, "\n")

	bottom := snakesim.GridH + 1 // top border + gridH grid rows
	if len(lines) <= bottom {
		t.Fatalf("frame has %d lines, want at least %d", len(lines), bottom+1)
	}
	if got := strings.TrimRight(lines[bottom], " "); len(got) == 0 || []rune(got)[0] != '╰' {
		t.Fatalf("row %d should be the board's bottom border, got %q", bottom, lines[bottom])
	}
	for i := bottom + 1; i < len(lines); i++ {
		if strings.ContainsAny(lines[i], "│╰╯╭╮─") {
			t.Fatalf("row %d below the board still has border glyphs: %q", i, lines[i])
		}
	}
}
