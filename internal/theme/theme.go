package theme

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	App             lipgloss.Style
	Header          lipgloss.Style
	Body            lipgloss.Style
	StatusInfo      lipgloss.Style
	StatusWarn      lipgloss.Style
	StatusError     lipgloss.Style
	Footer          lipgloss.Style
	SelectedTitle   lipgloss.Style
	SelectedSub     lipgloss.Style
	NormalTitle     lipgloss.Style
	NormalSub       lipgloss.Style
	Modal           lipgloss.Style
	ModalTitle      lipgloss.Style
	ModalHint       lipgloss.Style
	TabActive       lipgloss.Style
	TabInactive     lipgloss.Style
	InlineHighlight lipgloss.Style
}

func Dark() Styles {
	base := lipgloss.NewStyle().Background(lipgloss.Color("#0B0F10")).Foreground(lipgloss.Color("#E8EEF0"))

	return Styles{
		App:        base.Padding(0, 1),
		Header:     base.Background(lipgloss.Color("#12202A")).Foreground(lipgloss.Color("#D9EEF7")).Bold(true).Padding(0, 1),
		Body:       base,
		StatusInfo: base.Background(lipgloss.Color("#1F2D3A")).Foreground(lipgloss.Color("#9FD2FF")).Padding(0, 1),
		StatusWarn: base.Background(lipgloss.Color("#3A2E1F")).Foreground(lipgloss.Color("#FFD89F")).Padding(0, 1),
		StatusError: base.Background(lipgloss.Color("#3A1F22")).
			Foreground(lipgloss.Color("#FFB3B8")).Padding(0, 1),
		Footer:          base.Background(lipgloss.Color("#141A1E")).Foreground(lipgloss.Color("#A3B0B8")).Padding(0, 1),
		SelectedTitle:   base.Background(lipgloss.Color("#2B3942")).Foreground(lipgloss.Color("#FFFFFF")).Bold(true),
		SelectedSub:     base.Background(lipgloss.Color("#2B3942")).Foreground(lipgloss.Color("#B5C5CE")),
		NormalTitle:     base.Foreground(lipgloss.Color("#E8EEF0")),
		NormalSub:       base.Foreground(lipgloss.Color("#82939D")),
		Modal:           base.Background(lipgloss.Color("#0F1417")).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#6AA0C8")).Padding(1, 2),
		ModalTitle:      base.Foreground(lipgloss.Color("#D8F0FF")).Bold(true),
		ModalHint:       base.Foreground(lipgloss.Color("#8FA9B8")),
		TabActive:       base.Background(lipgloss.Color("#355067")).Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Padding(0, 1),
		TabInactive:     base.Background(lipgloss.Color("#1A252D")).Foreground(lipgloss.Color("#A3B0B8")).Padding(0, 1),
		InlineHighlight: base.Foreground(lipgloss.Color("#9FD2FF")).Bold(true),
	}
}
