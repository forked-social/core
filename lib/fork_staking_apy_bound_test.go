package lib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUpdateGlobalParamsStakingRewardsAPYBound proves the fork bound on the
// admin-tunable StakingRewardsAPYBasisPoints global param: values at or below
// MaxStakingRewardsAPYBasisPoints (25% = 2500 bps) are accepted, anything
// above is rejected without touching the stored entry, and an update that
// does not carry the key leaves the stored APY untouched. The fork's own
// default (DefaultStakingRewardsAPYBasisPoints) must sit under the bound.
func TestUpdateGlobalParamsStakingRewardsAPYBound(t *testing.T) {
	// The APY parse site is gated on ProofOfStake1StateSetupBlockHeight.
	setPoSBlockHeights(t, 2, 1000)

	require := require.New(t)

	chain, params, db := NewLowDifficultyBlockchain(t)
	mempool, miner := NewTestMiner(t, chain, params, true /*isSender*/)

	// Authorize the money key as a param updater.
	params.ExtraRegtestParamUpdaterKeys = make(map[PkMapKey]bool)
	params.ExtraRegtestParamUpdaterKeys[MakePkMapKey(MustBase58CheckDecode(moneyPkString))] = true

	// Mine past the PoS setup height.
	for ii := 0; ii < 10; ii++ {
		_, err := miner.MineAndProcessSingleBlock(0, mempool)
		require.NoError(err)
	}

	// Drop the minimum network fee to 1000 first: every PoS-era global params
	// update re-validates the fee-time-bucket sizing, which fails at the
	// chain's initial minimum fee (same setup as TestUpdateGlobalParamsPoS).
	_, _, _, err := _updateGlobalParamsEntryWithMempool(t, chain, db, params, 1000,
		moneyPkString, moneyPrivString,
		-1, 1000, -1, -1, -1, -1,
		map[string][]byte{}, true, mempool)
	require.NoError(err)

	updateAPY := func(apy uint64) error {
		_, _, _, err := _updateGlobalParamsEntryWithMempool(t, chain, db, params, 1000,
			moneyPkString, moneyPrivString,
			-1, -1, -1, -1, -1, -1,
			map[string][]byte{StakingRewardsAPYBasisPointsKey: UintToBuf(apy)},
			true, mempool)
		return err
	}

	// The fork default (2000 bps) must be admissible under the bound.
	require.LessOrEqual(ForkMainnetParams.DefaultStakingRewardsAPYBasisPoints,
		uint64(MaxStakingRewardsAPYBasisPoints))
	require.NoError(updateAPY(ForkMainnetParams.DefaultStakingRewardsAPYBasisPoints))
	require.Equal(ForkMainnetParams.DefaultStakingRewardsAPYBasisPoints,
		DbGetGlobalParamsEntry(db, chain.snapshot).StakingRewardsAPYBasisPoints)

	// The bound itself is inclusive.
	require.NoError(updateAPY(MaxStakingRewardsAPYBasisPoints))
	require.Equal(uint64(MaxStakingRewardsAPYBasisPoints),
		DbGetGlobalParamsEntry(db, chain.snapshot).StakingRewardsAPYBasisPoints)

	// One basis point over the bound is rejected, stored entry unchanged.
	err = updateAPY(MaxStakingRewardsAPYBasisPoints + 1)
	require.Error(err)
	require.ErrorIs(err, RuleErrorStakingRewardsAPYBasisPointsTooHigh)
	require.Equal(uint64(MaxStakingRewardsAPYBasisPoints),
		DbGetGlobalParamsEntry(db, chain.snapshot).StakingRewardsAPYBasisPoints)

	// An update without the APY key succeeds and leaves it untouched.
	_, _, _, err = _updateGlobalParamsEntryWithMempool(t, chain, db, params, 1000,
		moneyPkString, moneyPrivString,
		-1, -1, -1, -1, -1, -1,
		map[string][]byte{StakingRewardsMaxNumStakesKey: UintToBuf(500)}, true, mempool)
	require.NoError(err)
	updated := DbGetGlobalParamsEntry(db, chain.snapshot)
	require.Equal(uint64(MaxStakingRewardsAPYBasisPoints), updated.StakingRewardsAPYBasisPoints)
	require.Equal(uint64(500), updated.StakingRewardsMaxNumStakes)
}
