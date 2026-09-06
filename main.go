package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	fps       = 20
	turnSteps = fps * 4 // one turn every four seconds
)

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second/fps, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type model struct {
	step  int // rotation, in turnSteps of a full turn
	label string
	draw  func(step int) string
}

func (m model) Init() tea.Cmd { return tick() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		m.step = (m.step + 1) % turnSteps
		return m, tick()
	case tea.KeyMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string {
	caption := lipgloss.NewStyle().
		Foreground(lipgloss.Color("225")).
		Width(cellsW).Align(lipgloss.Center).
		Render(m.label + dots(m.step))
	return "\n" + m.draw(m.step) + "\n" + caption + "\n"
}

func dots(step int) string {
	return []string{"", ".", "..", "..."}[step/5%4]
}

// memo replays whole frames. The animation is exactly periodic over one turn —
// the outlet pulses a whole number of times per turn — so each frame is drawn
// once and then reused, which is what makes the sixel path affordable.
func memo(draw func(t float64) string) func(step int) string {
	cache := make([]string, turnSteps)
	return func(step int) string {
		if cache[step] == "" {
			cache[step] = draw(2 * math.Pi * float64(step) / turnSteps)
		}
		return cache[step]
	}
}

func main() {
	how := flag.String("render", "auto", "auto, sixel, sextants or half")
	size := flag.Int("size", cellsW, "width of the furnace, in terminal cells")
	flag.Parse()
	setSize(*size)

	m := model{label: "charging master spark"}
	if args := flag.Args(); len(args) > 0 {
		m.label = args[0]
	}

	// Ask the terminal what it can do before Bubble Tea takes over stdin.
	term := probeTerminal(250 * time.Millisecond)
	if *how == "auto" {
		*how = "sextants"
		if term.sixel {
			*how = "sixel"
		}
	}
	switch *how {
	case "sixel":
		m.draw = memo(func(t float64) string { return sixelBlock(t, term.cell) })
	case "sextants":
		m.draw = memo(func(t float64) string { return Render(t, sextants) })
	case "half":
		m.draw = memo(func(t float64) string { return Render(t, halfBlocks) })
	default:
		fmt.Fprintf(os.Stderr, "unknown -render %q\n", *how)
		os.Exit(2)
	}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
