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
	case "b", "B":
		return color.New(color.FgBlack)

	case "g":
		return color.New(color.FgHiGreen)

	case "y":
		return color.New(color.FgHiYellow)

	case "r":
		return color.New(color.FgHiRed)

	case "G": // dark green
		return color.New(color.FgGreen)

	case "Y": // dark grey
		return color.New(color.FgHiBlack)

	case "R": // dark red
		return color.New(color.FgRed)

	case "w":
		return color.New(color.FgHiWhite)

	case "W": // light grey
		return color.New(color.FgWhite)
	}
	fmt.Printf("ERROR: unhandled color code '%s'\n", token)
	return color.New(color.FgWhite)
}
