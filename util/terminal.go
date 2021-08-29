package util

import (
	"github.com/charmbracelet/lipgloss"
	"strings"
)

var (
	border = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "└",
		BottomRight: "┘",
	}
)

func RenderTable(mapping map[string]string, width int) string {
	keys := make([]string, 0, len(mapping))
	values := make([]string, 0, len(mapping))

	for k, v := range mapping {
		keys = append(keys, k)
		values = append(values, v)
	}

	titles := lipgloss.JoinVertical(
		lipgloss.Left,
		keys...,
	)

	data := lipgloss.JoinVertical(
		lipgloss.Right,
		values...,
	)

	spacer := strings.Repeat(" ", width-lipgloss.Width(titles)-lipgloss.Width(data))

	boxStyle := lipgloss.NewStyle().
		Width(width).
		Border(border, true, true, true, true).
		BorderForeground(lipgloss.Color("#43A9FF"))

	return boxStyle.Render(lipgloss.JoinHorizontal(lipgloss.Center, titles, spacer, data))
}
