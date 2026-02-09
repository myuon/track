package cli

import (
	"io"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

type cliColor struct {
	enabled bool
}

func newCLIColor(w io.Writer) cliColor {
	f, ok := w.(*os.File)
	if !ok {
		return cliColor{enabled: false}
	}
	if os.Getenv("NO_COLOR") != "" || strings.TrimSpace(os.Getenv("CLICOLOR")) == "0" {
		return cliColor{enabled: false}
	}
	fd := f.Fd()
	return cliColor{enabled: isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)}
}

func (c cliColor) status(v string) string {
	switch v {
	case "todo":
		return c.wrap("36", v)
	case "ready":
		return c.wrap("34", v)
	case "in_progress":
		return c.wrap("33", v)
	case "blocked":
		return c.wrap("31", v)
	case "done":
		return c.wrap("32", v)
	case "archived":
		return c.wrap("90", v)
	default:
		return c.wrap("35", v)
	}
}

func (c cliColor) priority(v string) string {
	switch v {
	case "p0":
		return c.wrap("1;31", v)
	case "p1":
		return c.wrap("31", v)
	case "p2":
		return c.wrap("33", v)
	case "p3":
		return c.wrap("36", v)
	case "none":
		return c.wrap("90", v)
	default:
		return c.wrap("35", v)
	}
}

func (c cliColor) wrap(code, v string) string {
	if !c.enabled {
		return v
	}
	return "\x1b[" + code + "m" + v + "\x1b[0m"
}
