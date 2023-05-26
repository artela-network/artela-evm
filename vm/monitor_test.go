package vm

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

func TestNewCommands(t *testing.T) {
	byteCode := "608080604052346100165761021c908161001b8239f35b5f80fdfe608060048036101561000f575f80fd5b5f91823560e01c63ad1c61fd14610024575f80fd5b3461018457604091826003193601126101805780359360249485359367ffffffffffffffff808611610180573660238701121561018057858501359581871161016e57601f1996601f81018816603f01881684018381118582101761015c578952808452368a82840101116101585789929181879260209c8d930183880137850101526568616861686160d01b896100ba610188565b6006815201528751918289019182118383101761014757508752828152870152919290916002e06002556002e06100ef610188565b928352636861686160e01b858401528351948592818452845191828186015281955b83871061012f5750508394508582601f949501015201168101030190f35b86810182015189880189015295810195889550610111565b634e487b7160e01b86526041875285fd5b8580fd5b634e487b7160e01b8752604188528a87fd5b634e487b7160e01b8552604186528885fd5b8380fd5b8280fd5b604051906040820182811067ffffffffffffffff8211176101a857604052565b634e487b7160e01b5f52604160045260245ffdfea2646970667358221220bdbd5e1cb169bd9dcadfb9de93f8478dd828d916d6ea28e6e6d2b07a98913a4864736f6c63782c302e382e32302d646576656c6f702e323032332e352e32352b636f6d6d69742e64636535386139382e6d6f64005d"
	input := "ad1c61fd0000000000000000000000000000000000000000000000000000000000000064000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000046861686100000000000000000000000000000000000000000000000000000000"
	statedb, _ := state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
	vmConf := Config{
		ExtraEips: []int{3855},
	}
	sender := common.Address{}
	sender.SetBytes([]byte("artela"))

	vmctx := BlockContext{
		Transfer: func(StateDB, common.Address, common.Address, *big.Int) {},
		CanTransfer: func(db StateDB, address common.Address, b *big.Int) bool {
			return true
		},
		BlockNumber: big0,
	}
	evm := NewEVM(vmctx, TxContext{}, statedb, params.AllEthashProtocolChanges, vmConf)
	_, address, _, err := evm.Create(AccountRef(sender), common.Hex2Bytes(byteCode), math.MaxUint64, new(big.Int))
	if err != nil {
		t.Error(err)
	}
	statedb.Finalise(true)

	evm = NewEVM(vmctx, TxContext{}, statedb, params.AllEthashProtocolChanges, vmConf)
	_, _, err = evm.Call(AccountRef(sender), address, common.Hex2Bytes(input), math.MaxUint64, new(big.Int))
	if err != nil {
		t.Error(err)
	}

	stateChanges := evm.interpreter.monitor.states
	stateChange, ok := stateChanges[address][*uint256.NewInt(0x2)][""]
	assert.True(t, ok, "state change not correct")
	assert.Equal(t, 2, len(stateChange), "state change not right")

	assert.True(t, new(uint256.Int).SetBytes(stateChange[0].Value).Eq(uint256.NewInt(0)), "state 0 value not eq")
	assert.True(t, new(uint256.Int).SetBytes(stateChange[1].Value).Eq(uint256.NewInt(100)), "state 1 value not eq")

	assert.True(t, bytes.Compare(stateChange[0].Account.Bytes(), common.Address{}.Bytes()) == 0, "state 0 account not eq")
	assert.True(t, bytes.Compare(stateChange[1].Account.Bytes(), sender.Bytes()) == 0, "state 1 account not eq")
}
