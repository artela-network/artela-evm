package vm

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

type State struct {
	Account common.Address // the account that triggered the state change
	Value   []byte         // raw data of the updated state
}

// Eq compares two states, return true if two states are equal
func (s *State) Eq(other *State) bool {
	return other != nil &&
		bytes.Compare(other.Account[:], s.Account[:]) == 0 &&
		bytes.Compare(other.Value, s.Value) == 0
}

// StateChanges saves the changes of current state
// the mapping is address -> slot -> index -> changes
type StateChanges map[common.Address]map[uint256.Int]map[string][]*State

func NewStateChanges() StateChanges {
	return make(map[common.Address]map[uint256.Int]map[string][]*State)
}

// Save saves a state change, if state already cached, skip the saving
func (s StateChanges) Save(account common.Address, slot *uint256.Int, index string, newState *State) {
	accountChange, ok := s[account]
	if !ok {
		s[account] = make(map[uint256.Int]map[string][]*State, 1)
		accountChange = s[account]
	}
	slotChange, ok := accountChange[*slot]
	if !ok {
		accountChange[*slot] = make(map[string][]*State, 1)
		slotChange = accountChange[*slot]
	}
	stateChange, ok := slotChange[index]
	if !ok {
		slotChange[index] = make([]*State, 0, 1)
		stateChange = slotChange[index]
	}

	// compare with last state change, skip if equal
	count := len(stateChange)
	if count > 0 && newState.Eq(stateChange[count-1]) {
		return
	}

	// initial state, state change account should be empty
	if count == 0 {
		newState.Account = common.Address{}
	}

	slotChange[index] = append(stateChange, newState)
}

// Monitor monitors the state changes and traces the call stack changes during a tx execution
type Monitor struct {
	states StateChanges
}

func NewMonitor() *Monitor {
	return &Monitor{states: NewStateChanges()}
}

func (m *Monitor) StateChanges() StateChanges {
	return m.states
}
