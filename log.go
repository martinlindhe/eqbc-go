package eqbc

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

func (eqbc *EQBC) Log(s string) {
	out := ""
	if !eqbc.noTimestamp {
		out = "[" + time.Now().Format("15:04:05") + "]"
	}
	out += colorize(s)
	fmt.Fprintf(color.Output, "%s\n", out)
}

func colorize(s string) string {
	for {
		pos := strings.Index(s, "[+")
		if pos == -1 {
			break
		}

		next := strings.Index(s, "+]")
		if next < pos {
			fmt.Printf("ERROR: invalid colorize string '%s'\n", s)
			return s
		}

		before := s[0:pos]
		after := s[next+2:]
		token := s[pos+2 : next]

		col := getColor(token)
		s = before + col.SprintFunc()(after)
	}

	return s
}

func getColor(token string) *color.Color {
	switch token {
	case "b":
		return color.New(color.FgHiBlack)

	case "x": // white
		return color.New(color.FgHiWhite)

	case "g":
		return color.New(color.FgHiGreen)

	case "y":
		return color.New(color.FgHiYellow)

	case "r":
		return color.New(color.FgHiRed)

	case "o":
		// XXX orange. not in ansi16
		return color.New(color.FgHiRed)

	case "u":
		return color.New(color.FgHiBlue)

	case "t":
		return color.New(color.FgHiCyan)

	case "m":
		return color.New(color.FgHiMagenta)

	case "p":
		// XXX purple. not in ansi16
		return color.New(color.FgHiMagenta)

	case "w":
		return color.New(color.FgHiWhite)

	case "B":
		return color.New(color.FgBlack)

	case "X": // white
		return color.New(color.FgHiWhite)

	case "G": // dark green
		return color.New(color.FgGreen)

	case "Y": // dark grey
		return color.New(color.FgHiBlack)

	case "R": // dark red
		return color.New(color.FgRed)

	case "O":
		// XXX dark orange. not in ansi16
		return color.New(color.FgRed)

	case "U":
		return color.New(color.FgBlue)

	case "T":
		return color.New(color.FgCyan)

	case "M":
		return color.New(color.FgMagenta)

	case "P":
		// XXX purple. not in ansi16
		return color.New(color.FgMagenta)

	case "W": // light grey
		return color.New(color.FgWhite)
	}
	fmt.Printf("ERROR: unhandled color code '%s'\n", token)
	return color.New(color.FgWhite)
}
