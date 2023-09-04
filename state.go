package greenlight

type phase string

type phaseKeyType string

const (
	phaseKey     phaseKeyType = "phase"
	phaseStartUp phase        = "startup"
	phaseRunning phase        = "running"
)

type numofCheckers int

type numofCheckersKeyType string

const (
	numofCheckersKey = numofCheckersKeyType("numofCheckers")
)

type State struct {
	Phase phase
}
