package greenlight

type Signal string

const (
	SignalNone   Signal = ""
	SignalGreen  Signal = "green"
	SignalYellow Signal = "yellow"
	SignalRed    Signal = "red"
)
