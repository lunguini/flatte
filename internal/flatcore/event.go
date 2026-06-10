package flatcore

import (
	"bufio"
	"io"
	"unicode/utf8"
)

type Key int

const (
	KeyUnknown Key = iota
	KeyUp
	KeyDown
	KeyEnter
	KeyCtrlC
	KeyBackspace
	KeyCharacter
	KeyTab
	KeyEscape
	KeyLeft
	KeyRight
	KeyDelete
	KeyResize
)

type Event struct {
	Key  Key
	Rune rune
}

func ReadEvent(reader io.Reader) (Event, error) {
	return readEvent(bufio.NewReader(reader))
}

func readEvent(reader *bufio.Reader) (Event, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return Event{}, err
	}

	switch b {
	case 3:
		return Event{Key: KeyCtrlC}, nil
	case '\r', '\n':
		return Event{Key: KeyEnter}, nil
	case '\t':
		return Event{Key: KeyTab}, nil
	case 127, 8:
		return Event{Key: KeyBackspace}, nil
	case 27:
		return readEscape(reader)
	default:
		if b < utf8.RuneSelf {
			return Event{Key: KeyCharacter, Rune: rune(b)}, nil
		}
		if err := reader.UnreadByte(); err != nil {
			return Event{}, err
		}
		r, _, err := reader.ReadRune()
		if err != nil {
			return Event{}, err
		}
		return Event{Key: KeyCharacter, Rune: r}, nil
	}
}

func readEscape(reader *bufio.Reader) (Event, error) {
	if reader.Buffered() == 0 {
		return Event{Key: KeyEscape}, nil
	}
	next, err := reader.Peek(1)
	if err != nil {
		return Event{Key: KeyEscape}, nil
	}
	if next[0] != '[' {
		return Event{Key: KeyEscape}, nil
	}
	if _, err := reader.ReadByte(); err != nil {
		return Event{}, err
	}

	code, err := reader.ReadByte()
	if err != nil {
		return Event{}, err
	}
	switch code {
	case 'A':
		return Event{Key: KeyUp}, nil
	case 'B':
		return Event{Key: KeyDown}, nil
	case 'C':
		return Event{Key: KeyRight}, nil
	case 'D':
		return Event{Key: KeyLeft}, nil
	case '3':
		tilde, err := reader.ReadByte()
		if err != nil {
			return Event{}, err
		}
		if tilde == '~' {
			return Event{Key: KeyDelete}, nil
		}
		return Event{Key: KeyUnknown}, nil
	default:
		return Event{Key: KeyUnknown}, nil
	}
}
