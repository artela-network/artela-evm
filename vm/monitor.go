package vm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

type State struct {
	Account common.Address // the account that triggered the state change
	Value   []byte         // raw data of the updated state
}

// StateChanges saves the changes of current state
// the mapping is address -> slot -> index -> changes
type StateChanges map[common.Address]map[uint256.Int]map[string][]*State

func (s StateChanges) Save(account common.Address, slot *uint256.Int, index string, newState *State) {
	changes, ok := s[account][*slot][index]
	if !ok {
		s[account][*slot][index] = make([]*State, 0, 1)
		changes = s[account][*slot][index]
	}
	changes = append(changes, newState)
}

// Monitor monitors the state changes and traces the call stack changes during a tx execution
type Monitor struct {
	states StateChanges
}

func (m *Monitor) StateChanges() StateChanges {
	return m.states
}
