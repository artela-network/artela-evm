package vm

import "github.com/holiman/uint256"

var (
	zero        = uint256.NewInt(0)
	one         = uint256.NewInt(1)
	two         = uint256.NewInt(2)
	eight       = uint256.NewInt(8)
	oneSlot     = uint256.NewInt(32)
	storageMask = uint256.NewInt(0xff)
)
