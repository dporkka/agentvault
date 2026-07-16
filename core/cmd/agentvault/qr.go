package main

import (
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/term"
)

// renderTerminalQR encodes s as a QR code and returns a string suitable for
// terminal output using Unicode half-block characters (▀, ▄, █, space).
// When the terminal is too narrow or is not a TTY, it returns an empty string
// with a descriptive error so the caller can fall back to printing the URL.
func renderTerminalQR(s string) (string, error) {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return "", errNotTTY
	}

	qr, err := qrcode.New(s, qrcode.Medium)
	if err != nil {
		return "", err
	}

	bitmap := qr.Bitmap()
	size := len(bitmap)
	qrWidth := size*2 + 2 // two cols per module + left/right border of 1 space

	termWidth, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termWidth < qrWidth {
		return "", errTermTooNarrow
	}

	var b strings.Builder
	b.Grow((size/2 + 3) * (qrWidth + 1))

	// Top border
	b.WriteByte('\n')
	for range qrWidth {
		b.WriteString("█")
	}
	b.WriteByte('\n')

	// Render two rows of QR modules at a time using half-block ▀
	for y := 0; y < size; y += 2 {
		b.WriteString("█") // left border
		for x := range size {
			upper := bitmap[y][x]
			lower := y+1 < size && bitmap[y+1][x]
			switch {
			case upper && lower:
				b.WriteString("█")
			case upper && !lower:
				b.WriteString("▀")
			case !upper && lower:
				b.WriteString("▄")
			default:
				b.WriteString(" ")
			}
		}
		b.WriteString("█\n") // right border + newline
	}

	// Bottom border
	for range qrWidth {
		b.WriteString("█")
	}
	b.WriteByte('\n')

	return b.String(), nil
}

// Sentinel errors so the caller can decide how to degrade.
var (
	errNotTTY        = &qrError{"stdout is not a terminal"}
	errTermTooNarrow = &qrError{"terminal too narrow for QR code"}
)

type qrError struct{ msg string }

func (e *qrError) Error() string { return e.msg }
