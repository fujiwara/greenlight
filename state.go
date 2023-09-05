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

type stateKeyType string

const (
	stateKey stateKeyType = "state"
)

type State struct {
	Phase      phase
	CheckIndex numofCheckers
}

func newState() *State {
	return &State{
		Phase:      phaseStartUp,
		CheckIndex: 0,
	}
}

func (s *State) NextPhase() {
	switch s.Phase {
	case phaseStartUp:
		s.Phase = phaseRunning
		s.CheckIndex = 0
	case phaseRunning:
		s.CheckIndex = 0
	}
}
