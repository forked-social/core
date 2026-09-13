package lib

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestForkGenesisParamsConsistency locks the fork genesis constants in
// fork_params.go to the genesis block as coded. validateParams in
// core/cmd/node.go fatals at node startup when these drift apart (it
// compares the embedded TransactionMerkleRoot / GenesisBlockHashHex
// against values computed from the genesis transactions), so any change
// to ForkGenesisBlock or ForkSeedBalances must recompute the constants.
//
// History: rotating MinerPubKeyBase58Check without recomputing the
// genesis merkle root shipped a node image that aborted on boot with
// "Genesis block merkle root ... not equal to computed merkle root ...".
// This test exists so that class of drift can never ship again.
//
// If this test fails after an intentional genesis change, re-run
//
//	go run ./scripts/fork_genesis -print
//
// and update ForkGenesisBlock.Header.TransactionMerkleRoot and
// ForkGenesisBlockHashHex in fork_params.go with the printed values.
func TestForkGenesisParamsConsistency(t *testing.T) {
	forkParamSets := map[string]*DeSoParams{
		"ForkMainnetParams": &ForkMainnetParams,
		"ForkTestnetParams": &ForkTestnetParams,
	}

	for name, params := range forkParamSets {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, params.GenesisBlock)
			require.NotEmpty(t, params.GenesisBlockHashHex)

			// Same check as validateParams (core/cmd/node.go): the embedded
			// merkle root must equal the root computed from the genesis
			// transactions as coded.
			merkle, _, err := ComputeMerkleRoot(params.GenesisBlock.Txns)
			require.NoError(t, err, "computing genesis merkle root")
			require.Equal(t,
				*params.GenesisBlock.Header.TransactionMerkleRoot, *merkle,
				"embedded TransactionMerkleRoot is stale: re-run "+
					"`go run ./scripts/fork_genesis -print` and update "+
					"fork_params.go after changing the genesis block")

			// Same check as validateParams: the embedded genesis hash must
			// equal the hash of the header as coded (the header embeds the
			// merkle root, so fixing one without the other fails here).
			genesisHash, err := params.GenesisBlock.Header.Hash()
			require.NoError(t, err, "hashing genesis header")
			require.Equal(t,
				params.GenesisBlockHashHex, hex.EncodeToString(genesisHash[:]),
				"embedded GenesisBlockHashHex is stale: re-run "+
					"`go run ./scripts/fork_genesis -print` and update "+
					"fork_params.go after changing the genesis block")

			// Documented allocation intent (backend/deploy.env and
			// backend/validators/README.md): the entire supply sits in a
			// single genesis output to the miner key.
			require.Len(t, params.SeedBalances, 1,
				"fork genesis allocation must be a single output")
			var totalNanos uint64
			for _, bal := range params.SeedBalances {
				totalNanos += bal.AmountNanos
			}
			require.Equal(t, uint64(MaxNanos), totalNanos,
				"fork genesis allocation must grant 100%% of the supply")
			require.Equal(t,
				MustBase58CheckDecode(MinerPubKeyBase58Check),
				params.SeedBalances[0].PublicKey,
				"fork genesis allocation must pay the miner key")
		})
	}
}
