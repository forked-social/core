// Command fork_genesis recomputes and verifies the genesis block identity of
// the forked-social network (core/lib/fork_params.go).
//
// Usage:
//
//	go run ./scripts/fork_genesis            # verify everything
//	go run ./scripts/fork_genesis -print     # print merkle root + header hash to embed
//
// If you modify ForkSeedBalances or ForkGenesisBlock in any way, re-run this
// tool with -print and update ForkGenesisBlockHex / ForkGenesisBlockHashHex in
// core/lib/fork_params.go accordingly.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/deso-protocol/core/lib"
)

var printValues = flag.Bool("print", false, "Print the computed merkle root and header hash instead of verifying")

func fail(format string, args ...interface{}) {
	fmt.Printf("FAIL: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	flag.Parse()

	// (c) Genesis balance check: the single SeedBalances output must sum to
	// exactly MaxCoinSupply and go to the miner key.
	var total uint64
	for _, bal := range lib.ForkSeedBalances {
		total += bal.AmountNanos
	}
	if len(lib.ForkSeedBalances) != 1 {
		fail("expected exactly 1 ForkSeedBalances output, got %d", len(lib.ForkSeedBalances))
	}
	if total != lib.MaxNanos {
		fail("ForkSeedBalances sum %d != MaxCoinSupply (MaxNanos) %d", total, lib.MaxNanos)
	}
	if lib.PkToString(lib.ForkSeedBalances[0].PublicKey, &lib.ForkMainnetParams) != lib.MinerPubKeyBase58Check {
		fail("ForkSeedBalances recipient is not the miner key %s", lib.MinerPubKeyBase58Check)
	}
	fmt.Printf("genesis balance: OK — single output of %d nanos (%d coins) to %s\n",
		total, total/lib.NanosPerUnit, lib.MinerPubKeyBase58Check)

	// Recompute the merkle root and header hash of the fork genesis block.
	merkle, _, err := lib.ComputeMerkleRoot(lib.ForkGenesisBlock.Txns)
	if err != nil {
		fail("ComputeMerkleRoot: %v", err)
	}
	headerHash, err := lib.ForkGenesisBlock.Header.Hash()
	if err != nil {
		fail("Header.Hash: %v", err)
	}
	merkleHex := hex.EncodeToString(merkle[:])
	hashHex := hex.EncodeToString(headerHash[:])
	fmt.Printf("merkle root: %s\nheader hash: %s\n", merkleHex, hashHex)

	if *printValues {
		return
	}

	// Validate that the embedded constants match the computed values.
	if *lib.ForkGenesisBlock.Header.TransactionMerkleRoot != *merkle {
		fail("embedded TransactionMerkleRoot %s does not match computed merkle %s — re-run with -print and update fork_params.go",
			hex.EncodeToString(lib.ForkGenesisBlock.Header.TransactionMerkleRoot[:]), merkleHex)
	}
	if lib.ForkGenesisBlockHashHex != hashHex {
		fail("embedded ForkGenesisBlockHashHex %s does not match computed hash %s — re-run with -print and update fork_params.go",
			lib.ForkGenesisBlockHashHex, hashHex)
	}
	fmt.Println("genesis merkle+hash: OK — embedded constants match computed values")

	// validateParams-equivalent checks for both fork variants (mirrors
	// core/cmd/node.go validateParams).
	for name, params := range map[string]*lib.DeSoParams{
		"ForkMainnetParams": &lib.ForkMainnetParams,
		"ForkTestnetParams": &lib.ForkTestnetParams,
	} {
		if params.BitcoinBurnAddress == "" {
			fail("%s: missing BitcoinBurnAddress", name)
		}
		numBlocks := params.TimeBetweenDifficultyRetargets / params.TimeBetweenBlocks
		if params.TimeBetweenBlocks*numBlocks != params.TimeBetweenDifficultyRetargets {
			fail("%s: TimeBetweenDifficultyRetargets must be divisible by TimeBetweenBlocks", name)
		}
		if params.GenesisBlock == nil || params.GenesisBlockHashHex == "" {
			fail("%s: missing genesis block info", name)
		}
		merkle, _, err := lib.ComputeMerkleRoot(params.GenesisBlock.Txns)
		if err != nil {
			fail("%s: could not compute genesis merkle root: %v", name, err)
		}
		if *merkle != *params.GenesisBlock.Header.TransactionMerkleRoot {
			fail("%s: genesis merkle root mismatch", name)
		}
		genesisHash, err := params.GenesisBlock.Header.Hash()
		if err != nil {
			fail("%s: problem hashing genesis header: %v", name, err)
		}
		if hex.EncodeToString(genesisHash[:]) != params.GenesisBlockHashHex {
			fail("%s: genesis hash mismatch (%s vs %s)", name,
				params.GenesisBlockHashHex, hex.EncodeToString(genesisHash[:]))
		}
		hexBytes, err := hex.DecodeString(params.MinDifficultyTargetHex)
		if err != nil || len(hexBytes) != 32 {
			fail("%s: invalid MinDifficultyTargetHex", name)
		}
		if params.MaxDifficultyRetargetFactor == 0 {
			fail("%s: MaxDifficultyRetargetFactor unset", name)
		}
		fmt.Printf("validateParams(%s): OK — network %s, prefixes %x, ports %d/%d, cutover %d, epoch %d\n",
			name, params.NetworkType, params.Base58PrefixPublicKey, params.DefaultSocketPort,
			params.DefaultJSONPort, params.ForkHeights.ProofOfStake2ConsensusCutoverBlockHeight,
			params.DefaultEpochDurationNumBlocks)
	}

	// Guard the ZERO-new-issuance invariant for the bootstrap window: the
	// block reward must be zero for every pre-cutover height.
	cutover := uint64(lib.ForkMainnetParams.ForkHeights.ProofOfStake2ConsensusCutoverBlockHeight)
	for hh := uint64(1); hh < cutover+5; hh++ {
		if rw := lib.CalcBlockRewardNanos(uint32(hh), &lib.ForkMainnetParams); rw != 0 {
			fail("PoW block reward at height %d is %d, expected 0 (suppressed)", hh, rw)
		}
	}
	fmt.Printf("block reward suppression: OK — rewards are zero for heights 1..%d\n", cutover+4)

	fmt.Printf("all fork genesis checks passed at %s\n", time.Now().UTC().Format(time.RFC3339))
}
