package lib

import (
	"math/big"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
)

// fork_params.go defines the chain parameters for the forked-social standalone
// network. The network is selected with the --forknet (mainnet-style) or
// --forknet-testnet flags (see core/cmd), which map onto DeSoParams defined in
// this file.
//
// Both variants reuse the existing NetworkType_MAINNET/NetworkType_TESTNET
// machinery internally; they only differ in their Base58 prefixes, ports, and
// default checkpoint provider, mirroring how upstream mainnet/testnet differ
// from one another.

// Keys for the fork network. Only public keys ever appear in code/config —
// corresponding seeds are injected at runtime via environment variables by the
// operator and are never committed.
const (
	// FounderRootPubKeyBase58Check is the governance root key of the fork. It
	// replaces the upstream "Architect" key as the sole param-updater, and it
	// is the admin/super-admin key for the seed node backend.
	FounderRootPubKeyBase58Check = "FS13vrbSsSRefiNeoJK5uHMKgCwzJJ3NMCFdEKpui7qPX4X7xSLSko"

	// MinerPubKeyBase58Check receives the entire ForkSeedSupplyNanos genesis
	// allocation (42% of the MaxNanos cap). It is the key used to mine the PoW
	// bootstrap blocks and funds the launch validators from that allocation.
	MinerPubKeyBase58Check = "FS13xKCghHsuC7Kw6eS1Y7cxePu3ZJ6PbYpxfaZzNcRKEQFXuFU2tp"

	// BlockProducerPubKeyBase58Check is the founder-operated block producer.
	BlockProducerPubKeyBase58Check = "FS13wbrugMsxmBzo41C1NGYUjCn4Tt4mQRKjzZAGa51wGCpKPJQ4cP"

	// StarterDeSoPubKeyBase58Check is the testnet faucet key. It is referenced
	// in deployment config only (the seed itself is runtime-injected).
	StarterDeSoPubKeyBase58Check = "FS13yCzGJGuQNmrsDn6Et74LphomuxnbsshZ3Rwo9k7iuxtLVRqyFV"
)

// ForkNetForkHeights defines fork heights for the fork network. It follows the
// RegtestForkHeights pattern: all legacy upgrades are considered applied at
// height 0, the balance model / lockups / PoS state setup activate at height 1
// (the first block after genesis), and consensus cuts over to PoS shortly
// after enough blocks have been mined for two full epochs to complete.
//
// Epoch snapshot timing (DefaultEpochDurationNumBlocks = 144):
//   - Height 1 (ProofOfStake1StateSetupBlockHeight): the epoch-complete hook
//     runs for the first time; epoch 1 spans blocks 2..145.
//   - Height 145: epoch 1 completes; its validator-set snapshot is taken.
//   - Height 289: epoch 2 completes; its snapshot is taken.
//   - Height 290..: epoch 3 uses the snapshot from epoch 1
//     (SnapshotLookbackNumEpochs = 2).
//
// The cutover height of 300 is >= 1 + 2*144 = 289 with margin, exactly as the
// regtest parameters are tuned. Validators must register and stake on-chain
// before the end of epoch 1 (block 145) to make it into the validator set used
// by the first PoS epoch; registrations after that join at the next epoch
// boundary after cutover. Validators are NOT genesis-baked — the params only
// need to permit registration, which they do from height 1 onward.
var ForkNetForkHeights = ForkHeights{
	DefaultHeight:                0,
	DeflationBombBlockHeight:     0,
	SalomonFixBlockHeight:        uint32(0),
	DeSoFounderRewardBlockHeight: uint32(0),
	BuyCreatorCoinAfterDeletedBalanceEntryFixBlockHeight: uint32(0),
	ParamUpdaterProfileUpdateFixBlockHeight:              uint32(0),
	UpdateProfileFixBlockHeight:                          uint32(0),
	BrokenNFTBidsFixBlockHeight:                          uint32(0),
	DeSoDiamondsBlockHeight:                              uint32(0),
	NFTTransferOrBurnAndDerivedKeysBlockHeight:           uint32(0),
	DeSoV3MessagesBlockHeight:                            uint32(0),
	BuyNowAndNFTSplitsBlockHeight:                        uint32(0),
	DAOCoinBlockHeight:                                   uint32(0),
	ExtraDataOnEntriesBlockHeight:                        uint32(0),
	DerivedKeySetSpendingLimitsBlockHeight:               uint32(0),
	DerivedKeyTrackSpendingLimitsBlockHeight:             uint32(0),
	DAOCoinLimitOrderBlockHeight:                         uint32(0),
	DerivedKeyEthSignatureCompatibilityBlockHeight:       uint32(0),
	OrderBookDBFetchOptimizationBlockHeight:              uint32(0),
	ParamUpdaterRefactorBlockHeight:                      uint32(0),
	DeSoUnlimitedDerivedKeysBlockHeight:                  uint32(0),
	AssociationsAndAccessGroupsBlockHeight:               uint32(0),
	AssociationsDerivedKeySpendingLimitBlockHeight:       uint32(0),

	// The genesis block was created using the utxo model, so the balance model
	// activates on the first mined block, matching the regtest setup.
	BalanceModelBlockHeight: uint32(1),
	LockupsBlockHeight:      uint32(1),

	ProofOfStake1StateSetupBlockHeight: uint32(1),

	// See the comment on ForkNetForkHeights for the epoch-snapshot reasoning
	// behind this height.
	ProofOfStake2ConsensusCutoverBlockHeight: uint32(300),

	BlockRewardPatchBlockHeight: uint32(0),

	// Be sure to update EncoderMigrationHeights as well via
	// GetEncoderMigrationHeights if you're modifying schema.
}

// ForkSeedSupplyNanos is the initial mint of the fork network: 420,000,000
// coins paid out in the genesis allocation. That is 42% of the MaxNanos cap;
// the remaining 58% is intentionally unallocated at genesis and exists only
// as headroom under the cap.
const ForkSeedSupplyNanos = uint64(420_000_000) * NanosPerUnit

// MaxStakingRewardsAPYBasisPoints bounds the admin-tunable
// StakingRewardsAPYBasisPoints global param at 25% so a founder-key
// UPDATE_GLOBAL_PARAMS mistake cannot mint at absurd rates. It is enforced in
// _connectUpdateGlobalParams.
const MaxStakingRewardsAPYBasisPoints = 2500

// ForkSeedBalances is the genesis allocation of the fork network: a single
// output granting ForkSeedSupplyNanos to the miner key. PoW block rewards are
// suppressed for the fork params (see DeSoParams.DisablePoWBlockRewards), so
// mining never mints; supply grows only through staking reward mints, which
// the protocol does not cap against MaxNanos (see the MaxNanos comment).
var ForkSeedBalances = []*DeSoOutput{
	{
		PublicKey:   MustBase58CheckDecode(MinerPubKeyBase58Check),
		AmountNanos: ForkSeedSupplyNanos,
	},
}

// ForkGenesisBlock is the genesis block of the fork network. It differs from
// the upstream genesis in its transactions (the fork seed balances) and its
// timestamp. Use core/scripts/fork_genesis to recompute the merkle root and
// header hash whenever this block is modified.
var ForkGenesisBlock = MsgDeSoBlock{
	Header: &MsgDeSoHeader{
		Version:               0,
		PrevBlockHash:         &BlockHash{},
		TransactionMerkleRoot: mustDecodeHexBlockHash("38b32569c4a9b9ad255b1bfb4bfcef372abd1fa44b4690930857d6af23fa8bf4"),
		// Sun Aug 23 2026 @ 00:00:00 UTC.
		TstampNanoSecs: SecondsToNanoSeconds(1787443200),
		Height:         uint64(0),
		Nonce:          uint64(0),
	},
	Txns: []*MsgDeSoTxn{
		{
			TxInputs: []*DeSoInput{},
			// The outputs in the genesis block aren't actually used by anything, but
			// including them helps our block explorer return the genesis transactions
			// without needing an explicit special case.
			TxOutputs: ForkSeedBalances,
			TxnMeta: &BlockRewardMetadataa{
				ExtraData: []byte("forked.social genesis block — Sun Aug 23 2026."),
			},
			// A signature is not required for BLOCK_REWARD transactions since they
			// don't spend anything.
		},
	},
}

// ForkGenesisBlockHashHex is the hash of ForkGenesisBlock. It is verified
// against the block at node boot by validateParams in core/cmd/node.go.
// Recompute with: go run ./scripts/fork_genesis -print
var ForkGenesisBlockHashHex = "77a061d6ca040bbf394c6d4bbd0ce219a1379b3a3e45bf36145b5dcd40ae6ad1"

var ForkGenesisBlockHash = mustDecodeHexBlockHash(ForkGenesisBlockHashHex)

// forkParamsCommon returns the DeSoParams fields shared by both fork variants.
// Variant-specific fields (network type, prefixes, ports, Bitcoin config) are
// set in the composite literals below.
func forkParamsCommon() DeSoParams {
	return DeSoParams{
		ProtocolVersion:    ProtocolVersion2,
		MinProtocolVersion: 1,
		UserAgent:          "Architect",

		// The fork network is seeded manually via the seed node at
		// node.forked.social / test.forked.social, so DNS bootstrapping is
		// intentionally empty. Peers connect with --connect-ips.
		DNSSeeds:          []string{},
		DNSSeedGenerators: [][]string{},

		GenesisBlock:        &ForkGenesisBlock,
		GenesisBlockHashHex: ForkGenesisBlockHashHex,

		// Start with the (easy) upstream testnet difficulty so the bootstrap
		// window can be mined quickly; difficulty retargets take over from
		// there.
		MinDifficultyTargetHex: "0090000000000000000000000000000000000000000000000000000000000000",
		// We do not require a minimum amount of accumulated work to consider
		// our chain valid.
		MinChainWorkHex: "0000000000000000000000000000000000000000000000000000000000000000",

		// The fork network is seeded by a solo miner that must mine from the
		// genesis tip with no peers (see MinerPubKeyBase58Check and the
		// empty DNSSeeds above). The genesis block is timestamped at
		// launch-prep time, which is arbitrarily far in the past relative to
		// whenever the seed first boots. If the tip were considered stale
		// (as on mainnet/testnet, which rely on peers to refresh the tip),
		// chainState() would permanently report SyncStateSyncingHeaders and
		// DeSoMiner._startThread would silently wait forever without
		// mining. Mirror the upstream regtest precedent in
		// DeSoParams.EnableRegtest ("Make sure we don't care about
		// blockchain tip age") so the tip age never gates the seed.
		MaxTipAgePoW: 1000000 * time.Hour,
		// After the PoS cutover the tip refreshes continuously while the
		// validator set produces blocks, so the peer-network default is
		// safe (and conservative) here.
		MaxTipAgePoS: 24 * time.Hour,

		BitcoinExchangeFeeBasisPoints: 10,
		BitcoinDoubleSpendWaitSeconds: 5.0,
		ServerMessageChannelSize:      uint32(100),

		// The purchase position at genesis sits at the full MaxNanos cap,
		// which places the BitcoinExchange schedule astronomically past the
		// start of the burn curve (~2^1000 tranches), so effectively no new
		// coins can ever be created via BitcoinExchange.
		DeSoNanosPurchasedAtGenesis: MaxNanos,

		DialTimeout:               30 * time.Second,
		VersionNegotiationTimeout: 30 * time.Second,
		VerackNegotiationTimeout:  30 * time.Second,

		MaxAddressesToBroadcast: 10,

		BlockRewardMaturity: 5 * time.Minute,

		V1DifficultyAdjustmentFactor: 10,

		// Reject blocks that are more than two hours in the future.
		MaxTstampOffsetSeconds: 2 * 60 * 60,

		MaxBlockSizeBytesPoW:   1000000,
		MinerMaxBlockSizeBytes: 1000000,

		// This is a standalone network with no legacy balances, so mining 2s
		// targets (the regtest precedent) keep the ~300 block PoW bootstrap
		// window to roughly ten minutes.
		TimeBetweenBlocks:              2 * time.Second,
		TimeBetweenDifficultyRetargets: 6 * time.Second,
		MaxDifficultyRetargetFactor:    4,

		MiningIterationsPerCycle: 9500,

		DefaultPoWSnapshotBlockHeightPeriod: 1000,

		MaxUsernameLengthBytes:        MaxUsernameLengthBytes,
		MaxUserDescriptionLengthBytes: 20000,
		MaxProfilePicLengthBytes:      20000,
		MaxProfilePicDimensions:       100,
		MaxPrivateMessageLengthBytes:  10000,
		MaxNewMessageLengthBytes:      10000,

		StakeFeeBasisPoints:         10 * 100,
		MaxPostBodyLengthBytes:      50000,
		MaxPostSubLengthBytes:       140,
		MaxStakeMultipleBasisPoints: 10 * 100 * 100,
		MaxCreatorBasisPoints:       100 * 100,
		MaxNFTRoyaltyBasisPoints:    100 * 100,

		// The fork network has no seed transactions: all state is created
		// post-launch on-chain.
		SeedTxns: []string{},

		// The genesis allocation (ForkSeedSupplyNanos) pays 42% of the
		// MaxNanos cap to the miner key; the remainder stays unallocated.
		SeedBalances: ForkSeedBalances,

		// PoW block rewards are suppressed for the bootstrap window and every
		// height after it (see CalcBlockRewardNanos); staking rewards are the
		// network's only issuance source and mint uncapped relative to this
		// constant, so total supply may eventually exceed MaxNanos.
		DisablePoWBlockRewards:         true,
		ForkNetwork:                    true,
		CreatorCoinTradeFeeBasisPoints: 1,
		CreatorCoinSlope:               NewFloat().SetFloat64(0.003),
		CreatorCoinReserveRatio:        NewFloat().SetFloat64(0.3333333),

		CreatorCoinAutoSellThresholdNanos: uint64(10),

		DefaultStakeLockupEpochDuration:   uint64(3),
		DefaultValidatorJailEpochDuration: uint64(3),

		DefaultLeaderScheduleMaxNumValidators: uint64(100),
		DefaultValidatorSetMaxNumValidators:   uint64(1000),
		DefaultStakingRewardsMaxNumStakes:     uint64(10000),

		// Staking rewards mint new supply at 20% APY by default, redeemable
		// from the first epoch end after the PoS cutover.
		DefaultStakingRewardsAPYBasisPoints: uint64(2000),

		// See the comment on ForkNetForkHeights: the cutover height of 300 is
		// tuned for 144-block epochs, exactly like regtest.
		DefaultEpochDurationNumBlocks: uint64(144),

		DefaultJailInactiveValidatorGracePeriodEpochs:         uint64(48),
		DefaultBlockTimestampDriftNanoSecs:                    (time.Minute * 10).Nanoseconds(),
		DefaultFeeBucketGrowthRateBasisPoints:                 uint64(1000),
		DefaultMaximumVestedIntersectionsPerLockupTransaction: 1000,
		DefaultMempoolMaxSizeBytes:                            3 * 1024 * 1024 * 1024, // 3GB
		DefaultMempoolFeeEstimatorNumMempoolBlocks:            1,
		DefaultMempoolFeeEstimatorNumPastBlocks:               50,

		DefaultMaxBlockSizeBytesPoS:     32000,
		DefaultSoftMaxBlockSizeBytesPoS: 16000,
		DefaultMaxTxnSizeBytesPoS:       25000,

		DefaultBlockProductionIntervalMillisecondsPoS: 1500,
		DefaultTimeoutIntervalMillisecondsPoS:         30000,

		HandshakeTimeoutMicroSeconds: uint64(900000000),

		DisableNetworkManagerRoutines: false,

		DefaultMempoolCongestionFactorBasisPoints:             uint64(9000),
		DefaultMempoolPastBlocksCongestionFactorBasisPoints:   uint64(9000),
		DefaultMempoolPriorityPercentileBasisPoints:           uint64(1000),
		DefaultMempoolPastBlocksPriorityPercentileBasisPoints: uint64(9000),

		ForkHeights:                 ForkNetForkHeights,
		EncoderMigrationHeights:     GetEncoderMigrationHeights(&ForkNetForkHeights),
		EncoderMigrationHeightsList: GetEncoderMigrationHeightsList(&ForkNetForkHeights),

		// Check reasonably often for the PoS transition given the short PoW
		// bootstrap window.
		FastHotStuffConsensusTransitionCheckDuration: 10 * time.Second,
	}
}

// ForkMainnetParams defines the mainnet-style variant of the fork network
// ("FS1" prefixes, protocol port 42000, API port 42001). Selected with
// --forknet.
var ForkMainnetParams = forkMainnetParams()

func forkMainnetParams() DeSoParams {
	params := forkParamsCommon()
	params.NetworkType = NetworkType_MAINNET
	params.BitcoinBtcdParams = &chaincfg.MainNetParams
	params.BitcoinBurnAddress = "1PuXkbwqqwzEYo9SPGyAihAge3e9Lc71b"

	// See comment in the upstream mainnet Bitcoin config for how this start
	// node was generated.
	params.BitcoinStartBlockNode = NewBlockNode(
		mustDecodeHexBlockHashBitcoin("000000000000000000092d577cc673bede24b6d7199ee69c67eeb46c18fc978c"),
		653184,
		_difficultyBitsToHash(386798414),
		big.NewInt(0),
		&MsgDeSoHeader{
			TstampNanoSecs: SecondsToNanoSeconds(1602950620),
			Height:         0,
		},
		StatusBitcoinHeaderValidated,
	)

	params.DefaultSocketPort = uint16(42000)
	params.DefaultJSONPort = uint16(42001)

	// Mainnet-style fork prefix "FS1" (bytes are byte-compatible with the
	// identity side and must not change).
	params.Base58PrefixPublicKey = [3]byte{0x05, 0x01, 0xED}
	params.Base58PrefixPrivateKey = [3]byte{0x35, 0x00, 0x00}

	return params
}

// ForkTestnetParams defines the testnet-style variant of the fork network
// ("tFS" prefixes, protocol port 42420, API port 42421). Selected with
// --forknet-testnet.
var ForkTestnetParams = forkTestnetParams()

func forkTestnetParams() DeSoParams {
	params := forkParamsCommon()
	params.NetworkType = NetworkType_TESTNET
	params.MinProtocolVersion = 0
	params.BitcoinBtcdParams = &chaincfg.TestNet3Params
	params.BitcoinBurnAddress = "mhziDsPWSMwUqvZkVdKY92CjesziGP3wHL"

	params.BitcoinStartBlockNode = NewBlockNode(
		mustDecodeHexBlockHashBitcoin("000000000000003aae8fb976056413aa1d863eb5bee381ff16c9642283b1da1a"),
		1897056,
		_difficultyBitsToHash(424073553),
		big.NewInt(0),
		&MsgDeSoHeader{
			TstampNanoSecs: SecondsToNanoSeconds(1607659152),
			Height:         0,
		},
		StatusBitcoinHeaderValidated,
	)

	params.DefaultSocketPort = uint16(42420)
	params.DefaultJSONPort = uint16(42421)

	// Testnet-style fork prefix "tFS" (bytes are byte-compatible with the
	// identity side and must not change).
	params.Base58PrefixPublicKey = [3]byte{0x11, 0xC8, 0x7D}
	params.Base58PrefixPrivateKey = [3]byte{0x4f, 0x06, 0x1b}

	return params
}
