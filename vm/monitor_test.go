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
	byteCode := "60808060405234610016576103c2908161001b8239f35b5f80fdfe60406080815260049081361015610014575f80fd5b5f91823560e01c63ad1c61fd14610029575f80fd5b3461032a578160031936011261032a57803590602493843567ffffffffffffffff9384821161032a573660238301121561032a578184013585811161032657368882850101116103265761007b61032e565b97600689526568616861686160d01b6020809a0152875192888401848110898211176103145789528084528851601f1998601f850195908a8716603f018b16830190811183821017610302578b528482528b9085888601848401378882878501015201526100e761032e565b90600e82526d4d79546f6b656e2e64756d6d793360901b8b83015281602e6003e0600355602e6003e061011861032e565b93601085526f44756d6d7944756d6d792e64756d6d7960801b8a86015284603087e285546001948582811c921680156102f8575b8c8310146102e657601f8211610286575b50508590601f84116001146102005791839491849388956101f3575b5050501b915f199060031b1c1916178255929091925b603083e261019b61032e565b928352636861686160e01b858401528351948592818452845191828186015281955b8387106101db5750508394508582601f949501015201168101030190f35b868101820151898801890152958101958895506101bd565b01013592505f8080610179565b8680527f290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e56392909184808b16898e5b8983831061026c5750505010610251575b50505050811b0182559290919261018f565b5f1960f88660031b161c19920101351690555f80808061023f565b85880187013589559097019693840193889350018e61022e565b8780527f290decd9548b62a8d60345a988386fc84ba6bc95484008f6362f93160ef3e5639060051c8101918c86106102dc575b601f0160051c019085905b8281106102d1575061015d565b8881550185906102c4565b90915081906102b9565b634e487b7160e01b8852602289528388fd5b91607f169161014c565b634e487b7160e01b895260418a528489fd5b634e487b7160e01b8752604188528287fd5b8380fd5b8280fd5b604051906040820182811067ffffffffffffffff82111761034e57604052565b634e487b7160e01b5f52604160045260245ffdfea2646970667358221220dbfb884e2f89ec7c583ecfd224de250c7b5781de15cb8a13f3fd082b97dbc74e64736f6c63782c302e382e32302d646576656c6f702e323032332e352e33302b636f6d6d69742e35306363623835642e6d6f64005d"
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

	stateChanges := evm.Monitor().StateChanges()

	stateChange1 := stateChanges.Variable(address, "MyToken.dummy3", "")
	assert.Equal(t, 2, len(stateChange1), "state change not right")

	assert.True(t, new(uint256.Int).SetBytes(stateChange1[0].Value).Eq(uint256.NewInt(0)), "state 0 value not eq")
	assert.True(t, new(uint256.Int).SetBytes(stateChange1[1].Value).Eq(uint256.NewInt(100)), "state 1 value not eq")

	assert.True(t, bytes.Compare(stateChange1[0].Account.Bytes(), common.Address{}.Bytes()) == 0, "state 0 account not eq")
	assert.True(t, bytes.Compare(stateChange1[1].Account.Bytes(), sender.Bytes()) == 0, "state 1 account not eq")

	stateChange2 := stateChanges.Variable(address, "DummyDummy.dummy", "")
	assert.Equal(t, 2, len(stateChange2), "state change not right")

	assert.Equal(t, "", string(stateChange2[0].Value))
	assert.Equal(t, "haha", string(stateChange2[1].Value))

	assert.True(t, bytes.Compare(stateChange2[0].Account.Bytes(), common.Address{}.Bytes()) == 0, "state 0 account not eq")
	assert.True(t, bytes.Compare(stateChange2[1].Account.Bytes(), sender.Bytes()) == 0, "state 1 account not eq")
}
