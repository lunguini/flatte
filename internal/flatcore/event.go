package flatcore

import (
	"bufio"
	"io"
	"unicode/utf8"
)

func ReadEvent(reader io.Reader) (Event, error) {
	return readEvent(bufio.NewReader(reader))
}

func readEvent(reader *bufio.Reader) (Event, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return KeyEvent{}, err
	}

	switch b {
	case 3:
		return KeyEvent{Key: KeyCtrlC}, nil
	case '\r', '\n':
		return KeyEvent{Key: KeyEnter}, nil
	case '\t':
		return KeyEvent{Key: KeyTab}, nil
	case 127, 8:
		return KeyEvent{Key: KeyBackspace}, nil
	case 27:
		return readEscape(reader)
	default:
		if b < utf8.RuneSelf {
			return KeyEvent{Key: KeyCharacter, Rune: rune(b)}, nil
		}
		if err := reader.UnreadByte(); err != nil {
			return KeyEvent{}, err
		}
		r, _, err := reader.ReadRune()
		if err != nil {
			return KeyEvent{}, err
		}
		return KeyEvent{Key: KeyCharacter, Rune: r}, nil
	}
}

func readEscape(reader *bufio.Reader) (Event, error) {
	if reader.Buffered() == 0 {
		return KeyEvent{Key: KeyEscape}, nil
	}
	next, err := reader.Peek(1)
	if err != nil {
		return KeyEvent{Key: KeyEscape}, nil
	}
	if next[0] != '[' {
		return KeyEvent{Key: KeyEscape}, nil
	}
	if _, err := reader.ReadByte(); err != nil {
		return KeyEvent{}, err
	}

	code, err := reader.ReadByte()
	if err != nil {
		return KeyEvent{}, err
	}
	switch code {
	case 'A':
		return KeyEvent{Key: KeyUp}, nil
	case 'B':
		return KeyEvent{Key: KeyDown}, nil
	case 'C':
		return KeyEvent{Key: KeyRight}, nil
	case 'D':
		return KeyEvent{Key: KeyLeft}, nil
	case '3':
		tilde, err := reader.ReadByte()
		if err != nil {
			return KeyEvent{}, err
		}
		if tilde == '~' {
			return KeyEvent{Key: KeyDelete}, nil
		}
		return KeyEvent{Key: KeyUnknown}, nil
	default:
		return KeyEvent{Key: KeyUnknown}, nil
	}
}
