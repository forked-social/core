package lib

type DeSoNode struct {
	// Name of the node, displayed to users
	Name string

	// HTTPs URL to the node or app
	URL string

	// DeSo username of the node owner
	Owner string
}

//
// This list of nodes is maintained by the forked-social team. It previously
// tracked the upstream DeSo node directory; entries for upstream nodes were
// removed for the standalone fork network.
//
// When submitting a post, add the following to PostExtraData:
//   "Node": "ID"
//
// If your node is in the list then other nodes will be able to know where users
// are posting and can include a link to your node and give you free advertising.
//

var NODES = map[uint64]DeSoNode{
	1: {
		// The self-owned seed node of the fork mainnet.
		Name:  "ForkNet",
		URL:   "https://node.forked.social",
		Owner: "forked",
	},
	2: {
		// The self-owned seed node of the fork testnet.
		Name:  "ForkNet Testnet",
		URL:   "https://test.forked.social",
		Owner: "forked",
	},
}
