package vm

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"math/big"
)

const (
	// AccountBalanceMagic is the magic word we used to record the balance change of an account
	AccountBalanceMagic = ".balance"
)

type State struct {
	Account      common.Address // the account that triggered the state change
	InnerTxIndex uint64         // index of inner tx that is causing the state change
	Value        []byte         // raw data of the updated state
}

// Eq compares two states, return true if two states are equal
func (s *State) Eq(other *State) bool {
	return other != nil &&
		s.InnerTxIndex == other.InnerTxIndex &&
		bytes.Compare(other.Account[:], s.Account[:]) == 0 &&
		bytes.Compare(other.Value, s.Value) == 0
}

// StateChanges saves the changes of current state
// the mapping is address -> slot -> index -> changes
type StateChanges struct {
	slotIndex map[common.Address]map[uint256.Int]string
	changes   map[common.Address]map[string]map[string][]*State
}

// NewStateChanges create a new instance of state change cache
func NewStateChanges() *StateChanges {
	return &StateChanges{
		slotIndex: make(map[common.Address]map[uint256.Int]string),
		changes:   make(map[common.Address]map[string]map[string][]*State),
	}
}

// TransferWithRecord is a wrapper for transfer func with balance change monitor
func (s *StateChanges) TransferWithRecord(db StateDB, from, to common.Address, amount *big.Int, innerTx *InnerTransaction, transfer TransferFunc) {
	// When deploying a contract with EoA, innerTx could be nil
	innerTxIndex := uint64(0)
	if innerTx != nil {
		innerTxIndex = innerTx.Index()
	}

	s.saveBalance(from, common.Address{}, uint256.MustFromBig(db.GetBalance(from)), innerTxIndex)
	s.saveBalance(to, common.Address{}, uint256.MustFromBig(db.GetBalance(to)), innerTxIndex)
	transfer(db, from, to, amount)
	s.saveBalance(from, common.Address{}, uint256.MustFromBig(db.GetBalance(from)), innerTxIndex)
	s.saveBalance(to, common.Address{}, uint256.MustFromBig(db.GetBalance(to)), innerTxIndex)
}

// saveBalance saves the balance change of an account
func (s *StateChanges) saveBalance(account, caller common.Address, newBalance *uint256.Int, innerTxIndex uint64) {
	s.SaveState(account, AccountBalanceMagic, nil, "", &State{
		Account:      caller,
		Value:        newBalance.Bytes(),
		InnerTxIndex: innerTxIndex,
	})
}

// SaveState saves a state change, if state already cached, skip the saving
func (s *StateChanges) SaveState(account common.Address, stateVarName string, slot *uint256.Int, index string, newState *State) {
	if slot != nil {
		accountSlotIndex, ok := s.slotIndex[account]
		if !ok {
			s.slotIndex[account] = make(map[uint256.Int]string)
			accountSlotIndex = s.slotIndex[account]
		}
		if _, ok := accountSlotIndex[*slot]; !ok {
			accountSlotIndex[*slot] = stateVarName
		}
	}

	accountChange, ok := s.changes[account]
	if !ok {
		s.changes[account] = make(map[string]map[string][]*State, 1)
		accountChange = s.changes[account]
	}
	slotChange, ok := accountChange[stateVarName]
	if !ok {
		accountChange[stateVarName] = make(map[string][]*State, 1)
		slotChange = accountChange[stateVarName]
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
	return
}

// Balance looks up balance changes of an account
func (s *StateChanges) Balance(account common.Address) []*State {
	states, ok := s.changes[account][AccountBalanceMagic][""]
	if !ok {
		return nil
	}

	return states
}

// Variable looks up state changes by variable name
func (s *StateChanges) Variable(account common.Address, stateVarName string, index string) []*State {
	states, ok := s.changes[account][stateVarName][index]
	if !ok {
		return nil
	}

	return states
}

// Slot looks up state changes by storage slot
func (s *StateChanges) Slot(account common.Address, slot *uint256.Int, index string) []*State {
	if slot == nil {
		return nil
	}
	stateVar, ok := s.slotIndex[account][*slot]
	if !ok {
		return nil
	}

	return s.Variable(account, stateVar, index)
}

// IndicesOfChanges returns a collection of the change indices
func (s *StateChanges) IndicesOfChanges(account common.Address, stateVarName string) []string {
	accountChange, ok := s.changes[account]
	if !ok {
		return nil
	}

	stateVarChange, ok := accountChange[stateVarName]
	if !ok {
		return nil
	}

	indices := make([]string, 0, len(stateVarChange))
	for index := range stateVarChange {
		indices = append(indices, index)
	}

	return indices
}

// InnerTransaction records the current contract call information
type InnerTransaction struct {
	From  common.Address
	To    common.Address
	Data  []byte
	Value *uint256.Int
	Gas   *uint256.Int

	index  uint64
	parent *InnerTransaction
}

// IsHead checks whether current inner transaction is the original transaction
func (it *InnerTransaction) IsHead() bool {
	return it.parent == nil
}

// Parent gets the parent of the inner transaction
// if transaction is the original transaction, its parent will be nil
func (it *InnerTransaction) Parent() *InnerTransaction {
	return it.parent
}

// Index returns the current index of inner transaction
func (it *InnerTransaction) Index() uint64 {
	return it.index
}

// CallStacks record the current smart contract call stack
type CallStacks struct {
	head    *InnerTransaction // head is the beginning of all inner transaction, same with original transaction
	current *InnerTransaction // current inner transaction
	count   uint64            // inner transaction count, used for inner tx index
}

// Push a new inner transaction to the current call stacks
func (c *CallStacks) Push(new *InnerTransaction) {
	if c.head == nil {
		c.head = new
	}

	new.parent = c.current
	new.index = c.count

	c.current = new
	c.count += 1
}

// Pop from an inner transaction, reset current to its parent
func (c *CallStacks) Pop() {
	if c.current == nil {
		return
	}
	c.current = c.current.parent
}

// Head returns the original transaction
func (c *CallStacks) Head() *InnerTransaction {
	return c.head
}

// Current returns the current inner transaction
func (c *CallStacks) Current() *InnerTransaction {
	return c.current
}

// ParentOf finds the parent inner tx of a given index
func (c *CallStacks) ParentOf(index uint64) *InnerTransaction {
	cursor := c.current
	for cursor != nil && cursor.index != index {
		cursor = cursor.parent
	}

	if cursor == nil {
		return nil
	}

	return cursor.parent
}

// Monitor monitors the state changes and traces the call stack changes during a tx execution
type Monitor struct {
	states     *StateChanges
	callstacks *CallStacks
}

// NewMonitor creates a new instance of monitor
func NewMonitor() *Monitor {
	return &Monitor{
		states:     NewStateChanges(),
		callstacks: &CallStacks{},
	}
}

// StateChanges returns all state changes
func (m *Monitor) StateChanges() *StateChanges {
	return m.states
}

// CallStacks returns the current call stacks
func (m *Monitor) CallStacks() *CallStacks {
	return m.callstacks
}
