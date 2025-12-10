package blockchain

import "crypto/sha256"

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 10/12/2025
 * Time: 13:41
 */

// MerkleTree is a binary tree structure where each leaf node is a hash of data,
// and each non-leaf node is a hash of its two child nodes
// This creates a cryptographic fingerprint of an entire data set that can be
// verified efficiently without needing the entire data set
type MerkleTree struct {
	RootNode *MerkleNode // The top hash (Merkle Root) that represents the entire data set
}

// MerkleNode represents a single node in the Merkle Tree
// Each node stores a cryptographic hash and pointers to its children
type MerkleNode struct {
	Left  *MerkleNode // Left child node (nil for leaf nodes)
	Right *MerkleNode // Right child node (nil for leaf nodes)
	Data  []byte      // SHA-256 hash stored in this node
}

// NewMerkleNode creates a new node in the Merkle Tree
// This function handles both leaf nodes (hashing raw data) and internal nodes (hashing child nodes)
func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	node := MerkleNode{}

	// LEAF NODE: If both children are nil, this is a leaf node
	// Leaf nodes hash the actual transaction/block data
	if left == nil && right == nil {
		// Hash the raw data (transaction, file, etc.) to create leaf hash
		hash := sha256.Sum256(data) // SHA-256 produces a 32-byte hash
		node.Data = hash[:]         // Convert fixed-size array to slice

		// INTERNAL NODE: If children exist, this is an internal node
		// Internal nodes hash the concatenation of their children's hashes
	} else {
		// Concatenate left and right child hashes
		// This creates a deterministic way to combine child hashes
		prevHashes := append(left.Data, right.Data...)

		// Hash the concatenated child hashes
		hash := sha256.Sum256(prevHashes)
		node.Data = hash[:]
	}

	// Store the child pointers (nil for leaf nodes)
	node.Left = left
	node.Right = right

	return &node
}

// NewMerkleTree constructs a complete Merkle Tree from an array of data blocks
// Each data block (transaction, file chunk, etc.) becomes a leaf in the tree
func NewMerkleTree(data [][]byte) *MerkleTree {
	var nodes []MerkleNode // Temporary slice to hold nodes at the current tree level

	// STEP 1: Ensure an even number of leaves for binary tree construction
	// Merkle Trees require an even number of nodes at each level
	// If odd, duplicate the last element (common convention in blockchains)
	if len(data)%2 != 0 {
		data = append(data, data[len(data)-1]) // Duplicate last element
		// In Bitcoin: This is called "balanced Merkle tree" construction
	}

	// STEP 2: Create leaf nodes (bottom level of the tree)
	// Each data block becomes a leaf node by hashing it
	for _, dat := range data {
		// Create a leaf node with no children, just hashed data
		node := NewMerkleNode(nil, nil, dat)
		nodes = append(nodes, *node)
	}

	// DEBUG: At this point, nodes contain all leaf nodes
	// Example with 4 transactions: nodes = [hash(tx1), hash(tx2), hash(tx3), hash(tx4)]

	// STEP 3: Build a tree from bottom up (leaf → root)
	// We keep building parent levels until we reach a single root node
	// Each iteration reduces the number of nodes by half

	for i := 0; i < len(data)/2; i++ {
		var level []MerkleNode // Nodes at the next higher level

		// Pair up nodes at the current level to create their parent nodes
		for j := 0; j < len(nodes); j += 2 {
			// Create a parent node that hashes its two children
			node := NewMerkleNode(&nodes[j], &nodes[j+1], nil)
			level = append(level, *node)
		}

		// Move up one level: current nodes become the new level we just created
		nodes = level

		// DEBUG Example with 4 leaves:
		// Iteration 1: leaves [L1, L2, L3, L4] → parents [P1(hash(L1+L2)), P2(hash(L3+L4))]
		// Iteration 2: parents/Branch [P1, P2] → root [R(hash(P1+P2))]
		// Done (len(nodes) = 1)
	}

	// STEP 4: Create the MerkleTree struct with the root node
	// The root node's hash is the Merkle Root - cryptographic fingerprint of all data
	tree := MerkleTree{&nodes[0]} // nodes[0] is the final root node

	return &tree
}
