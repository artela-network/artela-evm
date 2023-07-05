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

type MockMessage struct {
}

func (m MockMessage) From() common.Address {
	return common.Address{}
}

func (m MockMessage) To() *common.Address {
	return &common.Address{}
}

func (m MockMessage) GasPrice() *big.Int {
	return big0
}

func (m MockMessage) GasFeeCap() *big.Int {
	return big0
}

func (m MockMessage) GasTipCap() *big.Int {
	return big0
}

func (m MockMessage) Gas() uint64 {
	return 0
}

func (m MockMessage) Value() *big.Int {
	return big0
}

func (m MockMessage) Nonce() uint64 {
	return 0
}

func (m MockMessage) IsFake() bool {
	return true
}

func (m MockMessage) Data() []byte {
	return nil
}

func TestNewCommands(t *testing.T) {
	byteCode := "6080806040523461001657610438908161001b8239f35b5f80fdfe608060049081361015610010575f80fd5b5f90813560e01c63ad1c61fd14610025575f80fd5b346103a557604090816003193601126103a15783359060249485359267ffffffffffffffff9283851161039d573660238601121561039d57848301359380851161038b57601f1995601f958087018816603f018816840183811185821017610379578952808452368b8284010111610375578a9291818b9260209d8e930183880137850101526568616861686160d01b8a89516100c1816103a9565b6006815201528751888101818110838211176103635784918c918b5286815201528751826319dbdbd960e21b918281528b88820152208951610102816103a9565b600a81526909af25cc8eadadaf2f0f60b31b8d8201528a51928d80850152888c85015260608401526060835260c083019683881085891117610351579260e092819260aa8f8f978c6d26bcaa37b5b2b717323ab6b6bc9960911b99528585858585e18455e1610170876103a9565b600e8752015260029184602e84e283519182116103405782546001918282811c92168015610336575b8d831014610325575080888d92116102c2575b50508a9387831160011461023d578291602e9583928d94610232575b50501b915f199060031b1c19161781555be28351906101e6826103a9565b8152636861686160e01b868201528351948686528151918288880152815b83811061021f57505081860185015201168201829003019150f35b8181018901518882018801528801610204565b015192505f806101c8565b90929388831691858c527f405787fa12a823e0f2b7631cc41b3ba8828b3321ca811111fa75cd3aa3bb5ace928d8d905b8282106102ab5750509084602e97969594939210610293575b505050811b0181556101d9565b01515f1960f88460031b161c191690555f8080610286565b80888697829497870151815501960194019061026d565b848c52887f405787fa12a823e0f2b7631cc41b3ba8828b3321ca811111fa75cd3aa3bb5ace9181860160051c830193861061031c575b0160051c019082905b82811061031157508c91506101ac565b8c8155018290610301565b925081926102f8565b634e487b7160e01b8c52602288528bfd5b91607f1691610199565b634e487b7160e01b8a526041865289fd5b634e487b7160e01b8d5260418952858dfd5b634e487b7160e01b8b5260418752838bfd5b8980fd5b634e487b7160e01b8b52604187528b8bfd5b634e487b7160e01b8852604184528888fd5b8680fd5b8280fd5b5080fd5b6040810190811067ffffffffffffffff8211176103c557604052565b634e487b7160e01b5f52604160045260245ffdfea26469706673582212208b2466108e122fa54be2a3496af759357f7c686b85e340deb531fb2a1c222afb64736f6c63782b302e382e32302d646576656c6f702e323032332e362e322b636f6d6d69742e62633931386665352e6d6f64005c"
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
	evm := NewEVM(vmctx, TxContext{Msg: new(MockMessage)}, statedb, params.AllEthashProtocolChanges, vmConf)
	_, address, _, err := evm.Create(AccountRef(sender), common.Hex2Bytes(byteCode), math.MaxUint64, new(big.Int))
	if err != nil {
		t.Error(err)
	}
	statedb.Finalise(true)

	evm = NewEVM(vmctx, TxContext{Msg: new(MockMessage)}, statedb, params.AllEthashProtocolChanges, vmConf)
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
