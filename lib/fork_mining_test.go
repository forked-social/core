package lib

import (
	"testing"
	"time"

	chainlib "github.com/btcsuite/btcd/blockchain"
	"github.com/stretchr/testify/require"
)

// newForkMainnetBlockchainForTest builds a Blockchain from the fork mainnet
// params exactly as shipped (real genesis block, real difficulty, real fork
// heights), configured like the seed node: no trusted block producers and
// an (effectively) empty miner-side block-producer signing key. This mirrors
// backend/deploy.env, which sets neither TRUSTED_BLOCK_PRODUCER_PUBLIC_KEYS
// nor BLOCK_PRODUCER_SEED during the PoW bootstrap window.
func newForkMainnetBlockchainForTest(t *testing.T) (*Blockchain, *DeSoParams) {
	setupTestDeSoEncoder(t)
	AppendToMemLog(t, "START")
	ReadOnlyUtxoViewRegenerationIntervalTxns = 1

	db, _ := GetTestBadgerDb()
	timesource := chainlib.NewMedianTime()
	params := ForkMainnetParams

	// No snapshot: the seed's PoW bootstrap is a mining-only harness and the
	// PoW process-block path explicitly supports running without one (see
	// the bc.snapshot == nil early-exits in Blockchain.ProcessBlock).
	chain, err := NewBlockchain(
		nil /*trustedBlockProducerPublicKeys — unset on the seed*/,
		0 /*trustedBlockProducerStartHeight*/, 0 /*maxSyncBlockHeight*/,
		&params, timesource, db, nil /*postgres*/, NewEventManager(),
		nil /*snapshot*/, false /*archivalMode*/,
		nil /*checkpointSyncingProviders*/, MinBlockIndexSize)
	require.NoError(t, err)

	t.Cleanup(func() {
		AppendToMemLog(t, "CLEANUP_START")
		resetTestDeSoEncoder(t)
		CleanUpBadger(db)
		AppendToMemLog(t, "CLEANUP_END")
	})

	return chain, &params
}

// TestForkParamsMaxTipAgePoWUnbounded is a regression test for the silent
// no-blocks launch stall of the fork seed node.
//
// History: the fork params follow the upstream regtest mining model (2s
// blocks, 6s retargets, fork heights activating at block 1, PoS cutover at
// block 300) but initially kept the mainnet/testnet MaxTipAgePoW of 24h.
// The fork genesis block is timestamped at network launch-prep time (weeks
// before the seed first boots), so at boot the tip is older than
// MaxTipAgePoW. Blockchain.isTipCurrent then reports the tip as not current,
// chainState() returns SyncStateSyncingHeaders forever (there are no peers
// on the standalone fork network), and DeSoMiner._startThread silently
// sleeps in a loop waiting for SyncStateFullyCurrent — no block is ever
// mined and nothing is logged at any verbosity.
//
// Upstream regtest avoids exactly this by setting MaxTipAgePoW to
// 1000000 * time.Hour in DeSoParams.EnableRegtest ("Make sure we don't care
// about blockchain tip age"). The fork params must do the same, since the
// fork network is seeded by a solo miner that must mine from a stale
// genesis tip with no peers.
func TestForkParamsMaxTipAgePoWUnbounded(t *testing.T) {
	forkParamSets := map[string]*DeSoParams{
		"ForkMainnetParams": &ForkMainnetParams,
		"ForkTestnetParams": &ForkTestnetParams,
	}

	for name, params := range forkParamSets {
		t.Run(name, func(t *testing.T) {
			// The regtest precedent value is 1000000 hours (~114 years).
			// Anything shorter risks re-introducing the stall whenever the
			// seed is (re)booted more than MaxTipAgePoW after the genesis
			// timestamp.
			require.GreaterOrEqual(t,
				int64(params.MaxTipAgePoW), int64(1000000*time.Hour),
				"fork networks must not care about PoW tip age: the seed "+
					"mines from a stale genesis tip with no peers "+
					"(see DeSoParams.EnableRegtest for the upstream "+
					"precedent)")
		})
	}
}

// TestForkMainnetMinerProducesBlocksFromGenesis reproduces the launch
// scenario end-to-end: a fork-mainnet chain at genesis with no peers must
// consider itself sync-current (so the miner's sync-state gate in
// DeSoMiner._startThread passes) and the miner must advance the tip past
// the height-1 fork-activation hooks (BalanceModelBlockHeight,
// LockupsBlockHeight, ProofOfStake1StateSetupBlockHeight) within a
// bounded time. Pre-fix, the chain reports SyncStateSyncingHeaders at
// genesis and the miner produces nothing, forever, silently.
func TestForkMainnetMinerProducesBlocksFromGenesis(t *testing.T) {
	chain, params := newForkMainnetBlockchainForTest(t)

	// The precondition the miner gates on: at genesis, with no peers, the
	// chain must already be SyncStateFullyCurrent. This is the exact
	// assertion that failed pre-fix (SyncStateSyncingHeaders, because the
	// genesis tip was older than MaxTipAgePoW).
	require.Equal(t, SyncStateFullyCurrent, chain.ChainState(),
		"fork chain at genesis must be sync-current so the solo seed miner "+
			"is allowed to mine; got %s", chain.ChainState())

	// Build the mining stack the same way the node does: mempool, block
	// producer (no block-producer signing key — the seed leaves
	// BLOCK_PRODUCER_SEED empty during the PoW bootstrap), miner with the
	// fork miner public key.
	mempool := NewDeSoMempool(
		chain, 0 /*rateLimitFeeRateNanosPerKB*/, 0 /*minFeeRateNanosPerKB*/,
		"" /*blockCypherAPIKey*/, true /*runReadOnlyViewUpdater*/,
		"" /*dataDir*/, "" /*mempoolDumpDir*/, true /*useDefaultBadgerOptions*/)
	blockProducer, err := NewDeSoBlockProducer(
		0 /*minBlockUpdateIntervalSeconds*/, 10 /*maxBlockTemplatesToCache*/,
		"" /*blockProducerSeed — PoW blocks don't need producer signatures*/,
		mempool, chain, params, chain.postgres)
	require.NoError(t, err)

	miner, err := NewDeSoMiner(
		[]string{MinerPubKeyBase58Check}, 1 /*numThreads*/, blockProducer, params)
	require.NoError(t, err)

	t.Cleanup(func() {
		miner.Stop()
		blockProducer.Stop()
		if !mempool.stopped {
			mempool.Stop()
		}
		time.Sleep(100 * time.Millisecond)
	})

	// Start the miner's background threads exactly like DeSoNode does. The
	// threads gate on chainState() == SyncStateFullyCurrent before each
	// MineAndProcessSingleBlock call; pre-fix they spin silently here.
	miner.Start()

	// Block 1 must be found essentially instantly (difficulty target
	// 0x0090... ≈ 1-in-455 per hash). Mine past height 1 so the height-1
	// fork-activation hooks (balance model, lockups, PoS state setup /
	// epoch-complete hook) are exercised by connecting real blocks.
	//
	// Read the tip under the chain lock, the same way Server does when it
	// polls sync state (see server.go's _checkFastHotStuffConsensusStart).
	getTipHeight := func() uint64 {
		chain.ChainLock.RLock()
		defer chain.ChainLock.RUnlock()
		return uint64(chain.blockTip().Height)
	}
	require.Eventually(t, func() bool {
		return getTipHeight() >= 3
	}, 60*time.Second, 250*time.Millisecond,
		"fork miner did not advance the chain tip past genesis within 60s "+
			"(tip height: %d)", getTipHeight())

	require.GreaterOrEqual(t, getTipHeight(), uint64(3))
}
