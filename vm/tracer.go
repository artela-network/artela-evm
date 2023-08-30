package vm

import (
	"bytes"
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"math/big"
)

// StorageChanges contains the state changes of a storage slot
type StorageChanges struct {
	changes map[uint64][][]byte
}

// newStorageChange creates a new instance of storage change
func newStorageChange() *StorageChanges {
	return &StorageChanges{changes: make(map[uint64][][]byte, 1)}
}

// append a new change to the storage change
func (c *StorageChanges) append(callIdx uint64, newVal []byte) {
	changes, ok := c.changes[callIdx]
	if !ok {
		c.changes[callIdx] = make([][]byte, 0, 1)
	} else if len(changes) > 0 && bytes.Compare(changes[len(changes)-1], newVal) == 0 {
		// ignore identical change
		return
	}

	c.changes[callIdx] = append(changes, newVal)
}

// Changes returns the changes of a storage slot
func (c *StorageChanges) Changes() map[uint64][][]byte {
	return c.changes
}

// StorageKey contains the state meta info of a storage slot.
type StorageKey struct {
	slot          *uint256.Int
	offset        uint8
	children      map[uint256.Int]map[uint8]*StorageKey
	childrenIndex map[string]*StorageKey
	changes       *StorageChanges
	data          []byte
}

// NewBranchKey creates a new instance of branch storage key,
// branch storage key only stores the meta info of a storage slot,
// it does not store the actual changed values of the given storage slot.
func NewBranchKey(slot *uint256.Int, offset uint8, data []byte) *StorageKey {
	return &StorageKey{
		slot:          slot,
		offset:        offset,
		data:          data,
		children:      make(map[uint256.Int]map[uint8]*StorageKey),
		childrenIndex: make(map[string]*StorageKey),
	}
}

// NewRootKey creates a new instance of root storage key,
// root storage key is an empty instance of storage key, which does not have a certain slot or offset.
// It is used to store the top level account state.
// The data field for root key is the balance of the account.
func NewRootKey() *StorageKey {
	return &StorageKey{
		children:      make(map[uint256.Int]map[uint8]*StorageKey),
		childrenIndex: make(map[string]*StorageKey),
	}
}

// Slot returns the slot of the storage key
func (k *StorageKey) Slot() *uint256.Int {
	return k.slot
}

// Offset returns the offset of the storage key
func (k *StorageKey) Offset() uint8 {
	return k.offset
}

// AddChild adds a child storage key to current one
func (k *StorageKey) AddChild(child *StorageKey) (*StorageKey, error) {
	slot, offset := child.Slot(), child.Offset()
	if k.children[*slot] == nil {
		k.children[*slot] = make(map[uint8]*StorageKey)
	}

	storageKey := string(child.data)
	if k.childrenIndex[storageKey] == nil {
		k.childrenIndex[storageKey] = child
	}

	existing, ok := k.children[*slot][offset]
	if !ok {
		k.children[*slot][offset] = child
		return child, nil
	}

	return existing, nil
}

// JournalChanges saves the changes of current storage key
func (k *StorageKey) JournalChanges(callIdx uint64, newVal []byte) {
	if k.changes == nil {
		k.changes = newStorageChange()
	}

	k.changes.append(callIdx, newVal)
}

// StateChanges saves the changes of current state
type StateChanges struct {
	// roots holds the storage change roots of accounts
	roots map[common.Address]*StorageKey
	// index holds an index table for searching storage key by slot and offset
	index map[common.Address]map[uint256.Int]map[uint8]*StorageKey
	// raw holds all raw state changes, the tracer will not decode it, developers can decode it by themselves
	raw map[common.Address]map[uint256.Int]map[uint64]common.Hash
}

// NewStateChanges create a new instance of state change cache
func NewStateChanges() *StateChanges {
	return &StateChanges{
		roots: make(map[common.Address]*StorageKey),
		index: make(map[common.Address]map[uint256.Int]map[uint8]*StorageKey),
		raw:   make(map[common.Address]map[uint256.Int]map[uint64]common.Hash),
	}
}

// saveBalance saves the balance change of an account
func (s *StateChanges) saveBalance(account common.Address, newBalance *uint256.Int, callIdx uint64) {
	rootKey, ok := s.roots[account]
	if !ok {
		rootKey = NewRootKey()
		s.roots[account] = rootKey
	}
	rootKey.JournalChanges(callIdx, newBalance.Bytes())
}

// saveRawStateChange saves the raw state change of a slot.
func (s *StateChanges) saveRawStateChange(account common.Address, slot uint256.Int, callIdx uint64, val common.Hash) {
	if _, ok := s.raw[account]; !ok {
		s.raw[account] = make(map[uint256.Int]map[uint64]common.Hash)
	}
	if _, ok := s.raw[account][slot]; !ok {
		s.raw[account][slot] = make(map[uint64]common.Hash)
	}
	s.raw[account][slot][callIdx] = val
}

// saveKey saves a storage key to the state change tree
func (s *StateChanges) saveKey(account common.Address, parent, self, offset *uint256.Int, index []byte) (err error) {
	offsetU8 := uint8(0)
	if offset != nil {
		offsetU64, overflow := offset.Uint64WithOverflow()
		if overflow || offsetU64 > 31 {
			return errors.New("offset overflow")
		}
		offsetU8 = uint8(offsetU64)
	}

	var child *StorageKey
	if parent == nil {
		// dealing with top level state var
		if s.roots[account] == nil {
			s.roots[account] = NewRootKey()
		}
		child, err = s.roots[account].AddChild(NewBranchKey(self, offsetU8, index))
	} else {
		// dealing with nested state var
		parentKey := s.findKey(account, parent, 0)
		if parentKey == nil {
			return errors.New("unknown Parent storage key node")
		}

		child, err = parentKey.AddChild(NewBranchKey(self, offsetU8, index))
	}

	if err != nil {
		return
	}

	s.addKey(account, child.Slot(), child.Offset(), child)
	return
}

// saveChange saves a storage change to the state change tree
func (s *StateChanges) saveChange(account common.Address, self, offset *uint256.Int, callIdx uint64, newVal []byte) (err error) {
	offsetU8 := uint8(0)
	if offset != nil {
		offsetU64, overflow := offset.Uint64WithOverflow()
		if overflow || offsetU64 > 31 {
			return errors.New("offset overflow")
		}
		offsetU8 = uint8(offsetU64)
	}

	if s.roots[account] == nil {
		return errors.New("unknown account")
	}

	selfNode := s.findKey(account, self, offsetU8)
	if selfNode == nil {
		return errors.New("storage key node not found")
	}

	selfNode.JournalChanges(callIdx, newVal)
	return
}

// addKey adds a storage key to the index table
func (s *StateChanges) addKey(account common.Address, slot *uint256.Int, offset uint8, key *StorageKey) {
	if _, ok := s.index[account]; !ok {
		s.index[account] = make(map[uint256.Int]map[uint8]*StorageKey)
	}
	if _, ok := s.index[account][*slot]; !ok {
		s.index[account][*slot] = make(map[uint8]*StorageKey)
	}
	if s.index[account][*slot][offset] == nil {
		s.index[account][*slot][offset] = key
	}
}

// findKey finds a storage key from the index table
func (s *StateChanges) findKey(account common.Address, slot *uint256.Int, offset uint8) *StorageKey {
	if _, ok := s.index[account]; !ok {
		return nil
	}
	if _, ok := s.index[account][*slot]; !ok {
		return nil
	}
	return s.index[account][*slot][offset]
}

// Balance looks up balance changes of an account
func (s *StateChanges) Balance(account common.Address) *StorageChanges {
	if _, ok := s.roots[account]; !ok {
		return nil
	}

	return s.roots[account].changes
}

// findKeyIndices finds a storage key from the index table by indices
func (s *StateChanges) findKeyIndices(account common.Address, stateVarName string, indices ...[]byte) *StorageKey {
	rootKey, ok := s.roots[account]
	if !ok {
		return nil
	}

	cursor, ok := rootKey.childrenIndex[stateVarName]
	if !ok {
		return nil
	}

	for _, index := range indices {
		cursor, ok = cursor.childrenIndex[string(index)]
		if !ok {
			return nil
		}
	}

	return cursor
}

// Variable looks up state changes by variable name
func (s *StateChanges) Variable(account common.Address, stateVarName string, indices ...[]byte) *StorageChanges {
	key := s.findKeyIndices(account, stateVarName, indices...)
	if key == nil {
		return nil
	}

	return key.changes
}

// Slot looks up state changes by storage slot
func (s *StateChanges) Slot(account common.Address, slot, offset *uint256.Int) (*StorageChanges, error) {
	if slot == nil {
		return nil, errors.New("slot empty")
	}

	offsetU8 := uint8(0)
	if offset != nil {
		offsetU64, overflow := offset.Uint64WithOverflow()
		if overflow || offsetU64 > 31 {
			return nil, errors.New("offset overflow")
		}
		offsetU8 = uint8(offsetU64)
	}

	storageKey := s.index[account][*slot][offsetU8]
	if storageKey == nil {
		return nil, nil
	}
	return storageKey.changes, nil
}

// IndicesOfChanges returns a collection of the change indices
func (s *StateChanges) IndicesOfChanges(account common.Address, stateVarName string, indices ...[]byte) [][]byte {
	key := s.findKeyIndices(account, stateVarName, indices...)
	if key == nil {
		return nil
	}

	res := make([][]byte, 0, len(key.childrenIndex))
	if len(key.childrenIndex) > 0 {
		for index := range key.childrenIndex {
			res = append(res, []byte(index))
		}
	}

	return res
}

// Call records the current contract call information
type Call struct {
	From     common.Address `json:"from"`
	To       common.Address `json:"to"`
	Data     []byte         `json:"data"`
	Value    *uint256.Int   `json:"value"`
	Gas      *uint256.Int   `json:"gas"`
	Index    uint64         `json:"index"`
	Parent   *Call          `json:"parent"`
	Children []*Call        `json:"children"`
}

// IsRoot checks whether current call is the original call
func (c *Call) IsRoot() bool {
	return c.Parent == nil
}

// CallTree record the current smart contract call tree
type CallTree struct {
	head    *Call            // head is the beginning of all call, same with original transaction
	current *Call            // current call
	count   uint64           // call count, used for call Index
	lookup  map[uint64]*Call // lookup table for call Index
}

func NewCallTree() *CallTree {
	return &CallTree{
		lookup: make(map[uint64]*Call, 1),
	}
}

// add a new call to the current call tree
func (c *CallTree) add(from, to common.Address, data []byte, value, gas *uint256.Int) {
	newCall := &Call{
		From:  from,
		To:    to,
		Data:  data,
		Value: value,
		Gas:   gas,

		Parent: c.current,
		Index:  c.count,
	}

	if c.head == nil {
		c.head = newCall
	}

	if c.current != nil {
		c.current.Children = append(c.current.Children, newCall)
	}

	c.lookup[c.count] = newCall
	c.current = newCall

	c.count += 1
}

// exit from a call, reset current to its Parent
func (c *CallTree) exit() {
	if c.current == nil {
		return
	}
	c.current = c.current.Parent
}

// Head returns the call that initiated by the original transaction
func (c *CallTree) Head() *Call {
	return c.head
}

// Current returns the current call
func (c *CallTree) Current() *Call {
	return c.current
}

// ParentOf finds the Parent call of a given Index
func (c *CallTree) ParentOf(index uint64) *Call {
	node := c.lookup[index]
	if node == nil {
		return nil
	}

	return node.Parent
}

// ChildrenOf finds the Children of a given Index
func (c *CallTree) ChildrenOf(index uint64) []*Call {
	node := c.lookup[index]
	if node == nil {
		return nil
	}

	return node.Children
}

// Tracer traces the state changes and call stack changes during a tx execution
type Tracer struct {
	states   *StateChanges
	callTree *CallTree
}

// NewTracer creates a new instance of tracer
func NewTracer() *Tracer {
	return &Tracer{
		states:   NewStateChanges(),
		callTree: NewCallTree(),
	}
}

// StateChanges returns all state changes
func (t *Tracer) StateChanges() *StateChanges {
	return t.states
}

// SaveRawStateChange saves a raw state change
func (t *Tracer) SaveRawStateChange(account common.Address, slot uint256.Int, val common.Hash) {
	t.states.saveRawStateChange(account, slot, t.CurrentCallIndex(), val)
}

// SaveStateChange saves a state change of a given slot at given offset
func (t *Tracer) SaveStateChange(account common.Address, slot, offset *uint256.Int, newVal []byte) error {
	return t.states.saveChange(account, slot, offset, t.CurrentCallIndex(), newVal)
}

// SaveStateKey saves the relation between state variable to a storage slot
func (t *Tracer) SaveStateKey(account common.Address, parent, self, offset *uint256.Int, index []byte) error {
	return t.states.saveKey(account, parent, self, offset, index)
}

// SaveCall saves a call to call tree
func (t *Tracer) SaveCall(from, to common.Address, data []byte, value *uint256.Int, gas *uint256.Int) {
	t.callTree.add(from, to, data, value, gas)
}

// ExitCall exits from current call stack
func (t *Tracer) ExitCall() {
	t.callTree.exit()
}

// CallTree returns the current call tree
func (t *Tracer) CallTree() *CallTree {
	return t.callTree
}

// TransferWithRecord is a wrapper for transfer func with balance change tracer
func (t *Tracer) TransferWithRecord(db StateDB, from, to common.Address, amount *big.Int, transfer TransferFunc) {
	// When deploying a contract with EoA, innerTx could be nil
	callIdx := t.CurrentCallIndex()
	t.states.saveBalance(from, uint256.MustFromBig(db.GetBalance(from)), callIdx)
	t.states.saveBalance(to, uint256.MustFromBig(db.GetBalance(to)), callIdx)
	transfer(db, from, to, amount)
	t.states.saveBalance(from, uint256.MustFromBig(db.GetBalance(from)), callIdx)
	t.states.saveBalance(to, uint256.MustFromBig(db.GetBalance(to)), callIdx)
}

func (t *Tracer) CurrentCallIndex() uint64 {
	callIdx := uint64(0)
	if t.callTree.current != nil {
		callIdx = t.callTree.current.Index
	}
	return callIdx
}
