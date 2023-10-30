## 
<h1 align="center"> Artela-EVM </h1>

<div align="center">
  <a href="https://t.me/artela_official" target="_blank">
    <img alt="Telegram Chat" src="https://img.shields.io/badge/chat-telegram-blue?logo=telegram&chat">
  </a>
  <a href="https://twitter.com/Artela_Network" target="_blank">
    <img alt="Twitter Follow" src="https://img.shields.io/twitter/follow/Artela_Network">
  <a href="https://discord.gg/artela">
   <img src="https://img.shields.io/badge/chat-discord-green?logo=discord&chat" alt="Discord">
  </a>
  <a href="https://www.artela.network/">
   <img src="https://img.shields.io/badge/Artela%20Network-3282f8" alt="Artela Network">
  </a>
</div>


This is an enhanced version of Ethereum Virtual Machine(EVM), it provides the ability to:

- Track the state changes
- Track the call stacks

With the above 2 abilities, it will be easier for Artela Aspect developers to know the states of their dApps, which could reduce the security issues in some ways.

## Development

NewEVM generates a Artela VM from the provided Message fields and the chain parameters

```go
import  "github.com/artela-network/artela-evm/vm"

// NewEVM generates a Artela VM from the provided Message fields and the chain parameters
// (ChainConfig and module Params). It additionally sets the validator operator address as the
// coinbase address to make it available for the COINBASE opcode, even though there is no
// beneficiary of the coinbase txs (since we're not mining).
func (k *Keeper) NewEVM(
	ctx cosmos.Context,
	msg *core.Message,
	cfg *states.EVMConfig,
	tracer vm.EVMLogger,
	stateDB vm.StateDB,
) *vm.EVM {
	blockCtx := vm.BlockContext{
		CanTransfer: artcore.CanTransfer,
		Transfer:    artcore.Transfer,
		GetHash:     k.GetHashFn(ctx),
		Coinbase:    cfg.CoinBase,
		GasLimit:    artela.BlockGasLimit(ctx),
		BlockNumber: big.NewInt(ctx.BlockHeight()),
		Time:        uint64(ctx.BlockHeader().Time.Unix()),
		Difficulty:  big.NewInt(0), // unused. Only required in PoW context
		BaseFee:     cfg.BaseFee,
		Random:      nil, // not supported
	}

	txCtx := artcore.NewEVMTxContext(msg)
	if tracer == nil {
		tracer = k.Tracer(ctx, msg, cfg.ChainConfig)
	}
	vmConfig := k.VMConfig(ctx, msg, cfg, tracer)
	return vm.NewEVM(blockCtx, txCtx, stateDB, cfg.ChainConfig, vmConfig)
}
```

## How does it work
- Require your smart contract compile with [ASOLC](https://docs.artela.network/develop/advanced-concepts/asolc)， 

###  Trace.State 

- Record SSTORE operations.
- Unimplemented features
    - Not able to handle array.push and delete for now, since this is not an assignment, it’s a function call

        ```solidity
        string[] dummy
        
        function push() {
        	dummy.push("haha");
        	delete dummy[0];
        }
        ```

        - Extra Op Codes for entire array change:
            - `VAJOURNAL` : stateVar, key, slot, isDelete, memSize
            - `RAJOURNAL` : stateVar, key, slot, isDelete, memSize
        - `aave-v3-core/protocol/configuration/PoolAddressesProviderRegistry.sol`

            ```solidity
            /**
               * @notice Adds the addresses provider address to the list.
               * @param provider The address of the PoolAddressesProvider
               */
              function _addToAddressesProvidersList(address provider) internal {
                _addressesProvidersIndexes[provider] = _addressesProvidersList.length;
                _addressesProvidersList.push(provider);
              }
            ```

        - `aave-v3-core/protocol/libraries/logic/PoolLogic.sol`

            ```solidity
            /**
               * @notice Drop a reserve
               * @param reservesData The state of all the reserves
               * @param reservesList The addresses of all the active reserves
               * @param asset The address of the underlying asset of the reserve
               */
              function executeDropReserve(
                mapping(address => DataTypes.ReserveData) storage reservesData,
                mapping(uint256 => address) storage reservesList,
                address asset
              ) external {
                DataTypes.ReserveData storage reserve = reservesData[asset];
                ValidationLogic.validateDropReserve(reservesList, reserve, asset);
                reservesList[reservesData[asset].id] = address(0);
                delete reservesData[asset];
              }
            ```

    - Not able to handle local storage pointer assignment for now, e.g.:

        ```solidity
        contract {
        	mapping(string => mapping(uint256 => string)) dummy;
        	mapping(uint256 => mapping(uint256 => string)) dummy2;
        	
        	function xxx() {
        		mapping(uint256 => string) storage local = dummy["test"];
        		// in the AST we are missing the previous local 
        		// storage variable declaration, need to cache it
        		local[1] = "test2";
        		// passing storage pointer to another function
        		xxx2(local);
        		// passing storage pointer to another function with scope escaping
        		xxx2(dummy2[1]);
        	}
        
        	function xxx2(mapping(uint256 => string) storage param) {
        		param[0] = "test3";
        	}
        }
        ```

        - Extra Op Codes for record scope escaping:
            - `RPARENT` : stateVar, prefixKey, slot, memSize → parent : bytes32

                ```json
                // Monitor --> intermediate state assignment
                {
                	"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" : { // storage slot
                	  "stateVar": "dummy",
                		"prefixKey": "0xbbbb...."
                	}, 
                	"0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" : {
                		// ...
                	}
                }
                ```

            - `VPJOURNAL` : parent, key, slot, memSize
            - `RPJOURNAL` : parent, key, slot, memSize
            - `VAPJOURNAL` : parent, key, slot, isDelete, memSize
            - `RAPJOURNAL` : parent, key, slot, isDelete, memSize
        - `aave-v3-core/protocol/libraries/BorrowLogic.sol`

            ```solidity
            /**
               * @notice Implements the borrow feature. Borrowing allows users that provided collateral to draw liquidity from the
               * Aave protocol proportionally to their collateralization power. For isolated positions, it also increases the
               * isolated debt.
               * @dev  Emits the `Borrow()` event
               * @param reservesData The state of all the reserves
               * @param reservesList The addresses of all the active reserves
               * @param eModeCategories The configuration of all the efficiency mode categories
               * @param userConfig The user configuration mapping that tracks the supplied/borrowed assets
               * @param params The additional parameters needed to execute the borrow function
               */
              function executeBorrow(
                mapping(address => DataTypes.ReserveData) storage reservesData,
                mapping(uint256 => address) storage reservesList,
                mapping(uint8 => DataTypes.EModeCategory) storage eModeCategories,
                DataTypes.UserConfigurationMap storage userConfig,
                DataTypes.ExecuteBorrowParams memory params
              ) public {
                DataTypes.ReserveData storage reserve = reservesData[params.asset];
                DataTypes.ReserveCache memory reserveCache = reserve.cache();
            
                reserve.updateState(reserveCache);
            
                (
                  bool isolationModeActive,
                  address isolationModeCollateralAddress,
                  uint256 isolationModeDebtCeiling
                ) = userConfig.getIsolationModeState(reservesData, reservesList);
            
                ValidationLogic.validateBorrow(
                  reservesData,
                  reservesList,
                  eModeCategories,
                  DataTypes.ValidateBorrowParams({
                    reserveCache: reserveCache,
                    userConfig: userConfig,
                    asset: params.asset,
                    userAddress: params.onBehalfOf,
                    amount: params.amount,
                    interestRateMode: params.interestRateMode,
                    maxStableLoanPercent: params.maxStableRateBorrowSizePercent,
                    reservesCount: params.reservesCount,
                    oracle: params.oracle,
                    userEModeCategory: params.userEModeCategory,
                    priceOracleSentinel: params.priceOracleSentinel,
                    isolationModeActive: isolationModeActive,
                    isolationModeCollateralAddress: isolationModeCollateralAddress,
                    isolationModeDebtCeiling: isolationModeDebtCeiling
                  })
                );
            
                uint256 currentStableRate = 0;
                bool isFirstBorrowing = false;
            
                if (params.interestRateMode == DataTypes.InterestRateMode.STABLE) {
                  currentStableRate = reserve.currentStableBorrowRate;
            
                  (
                    isFirstBorrowing,
                    reserveCache.nextTotalStableDebt,
                    reserveCache.nextAvgStableBorrowRate
                  ) = IStableDebtToken(reserveCache.stableDebtTokenAddress).mint(
                    params.user,
                    params.onBehalfOf,
                    params.amount,
                    currentStableRate
                  );
                } else {
                  (isFirstBorrowing, reserveCache.nextScaledVariableDebt) = IVariableDebtToken(
                    reserveCache.variableDebtTokenAddress
                  ).mint(params.user, params.onBehalfOf, params.amount, reserveCache.nextVariableBorrowIndex);
                }
            
                if (isFirstBorrowing) {
                  userConfig.setBorrowing(reserve.id, true);
                }
            
                if (isolationModeActive) {
                  uint256 nextIsolationModeTotalDebt = reservesData[isolationModeCollateralAddress]
                    .isolationModeTotalDebt += (params.amount /
                    10 **
                      (reserveCache.reserveConfiguration.getDecimals() -
                        ReserveConfiguration.DEBT_CEILING_DECIMALS)).toUint128();
                  emit IsolationModeTotalDebtUpdated(
                    isolationModeCollateralAddress,
                    nextIsolationModeTotalDebt
                  );
                }
            
                reserve.updateInterestRates(
                  reserveCache,
                  params.asset,
                  0,
                  params.releaseUnderlying ? params.amount : 0
                );
            
                if (params.releaseUnderlying) {
                  IAToken(reserveCache.aTokenAddress).transferUnderlyingTo(params.user, params.amount);
                }
            
                emit Borrow(
                  params.asset,
                  params.user,
                  params.onBehalfOf,
                  params.amount,
                  params.interestRateMode,
                  params.interestRateMode == DataTypes.InterestRateMode.STABLE
                    ? currentStableRate
                    : reserve.currentVariableBorrowRate,
                  params.referralCode
                );
              }
            ```

    - Not able to handle expression index, e.g.:

        ```solidity
        contract {
        	mapping(bytes8 => string) dummy;
        	mapping(bytes => string) a;
        	
        	function xxx(bytes calldata good) {
        		// uint8(1) will be recognized as a type conversion function call
        		dummy[bytes8(1)] = "haha";
        		// need to generate ir code for evaluating expression first
        		a[d == 0 ? good[2:] : good[0:2]] = "haha";
        	}
        }
        ```

        - `aave-v3-core/misc/AaveOracle.sol`

            ```solidity
            mapping(address => AggregatorInterface) private assetsSources;
            
            function _setAssetsSources(address[] memory assets, address[] memory sources) internal {
                require(assets.length == sources.length, Errors.INCONSISTENT_PARAMS_LENGTH);
                for (uint256 i = 0; i < assets.length; i++) {
                  assetsSources[assets[i]] = AggregatorInterface(sources[i]);
                  emit AssetSourceUpdated(assets[i], sources[i]);
                }
              }
            ```

        - `aave-v3-core/protocol/pool/Pool.sol`

            ```solidity
            /// @inheritdoc IPool
              function getReservesList() external view virtual override returns (address[] memory) {
                uint256 reservesListCount = _reservesCount;
                uint256 droppedReservesCount = 0;
                address[] memory reservesList = new address[](reservesListCount);
            
                for (uint256 i = 0; i < reservesListCount; i++) {
                  if (_reservesList[i] != address(0)) {
                    reservesList[i - droppedReservesCount] = _reservesList[i];
                  } else {
                    droppedReservesCount++;
                  }
                }
            
                // Reduces the length of the reserves array by `droppedReservesCount`
                assembly {
                  mstore(reservesList, sub(reservesListCount, droppedReservesCount))
                }
                return reservesList;
            }
            ```

        - `aave-v3-core/protocol/libraries/logic/PoolLogic.sol`

            ```solidity
            /**
               * @notice Drop a reserve
               * @param reservesData The state of all the reserves
               * @param reservesList The addresses of all the active reserves
               * @param asset The address of the underlying asset of the reserve
               */
              function executeDropReserve(
                mapping(address => DataTypes.ReserveData) storage reservesData,
                mapping(uint256 => address) storage reservesList,
                address asset
              ) external {
                DataTypes.ReserveData storage reserve = reservesData[asset];
                ValidationLogic.validateDropReserve(reservesList, reserve, asset);
                reservesList[reservesData[asset].id] = address(0);
                delete reservesData[asset];
              }
            ```

    - Not able to handle non-equal assignment, e.g.:

        ```solidity
        contract {
        	uint256 dummy;
        	
        	function xxx() {
        		dummy++;
        		dummy<<1;
        	}
        }
        ```

    - Not able to handle tuple assignment, e.g.:

        ```solidity
        contract {
        	uint256 dummy;
        	string dummy2;
        	
        	function xxx() {
        		(dummy, dummy2) = (1, "haha");
        	}
        }
        ```

    - EVM memory size calculation is not correct, need fix.
- Gas rule implementation for `JOURNAL` commands.
- Fix optimizer occasionally delete journal variable issue.

### Trace CallStack

CallStack Data

```go
type InnerTransaction struct {
	From  common.Address
	To    common.Address
	Data  []byte
	Value *uint256.Int
	Gas   *uint256.Int

	index  uint64
	parent *InnerTransaction
	children []*InnerTransaction
}
```

```json
{
	"from": "0xaaaaa...",         // <-- caller address
	"to": "0xbbbbb...",           // <-- contract address
	"data": "0xaaaaabbbbbcccc..." // <-- abi.encoded
	"value": 100000000,           // <-- value in wei
	"gas": 10000000,              // <-- amount
	"index": 1,                   // <-- inner tx index
	"parent": { ... }             // <-- pointer to parent call
	"children": [
		{ ... },
		{ ... }
	]
}

```

- Currently call input is still in binary format, need to decode it in Aspect

    ```tsx
    // add util function
    let params : ethereum.Value[] = ethereum.decode('0xaaaa....', '{method}({type1},{type2}...)');
    // decode with abi
    let decoder = new ethereum.AbiDecoder(abiJson);
    let method, params = decoder.decode('0xaaaaa');
    ```


## License
Copyright © Artela Network, Inc. All rights reserved.

Licensed under the [Apache v2](LICENSE) License.