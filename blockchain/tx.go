package blockchain

import (
	"bytes"

	"github.com/golang-blockchain/wallet"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 04/12/2025
 * Time: 16:45
 */

// TxOutput represents an unspent transaction output (UTXO)
// Think of it as a "locked box" of coins that can only be opened by the rightful owner
// Each output contains value and is locked to a specific owner's public key hash
type TxOutput struct {
	Value      int    // Amount of cryptocurrency/tokens in this output
	PubKeyHash []byte // Hash of the owner's public key (the "lock")
	// Owner can be:
	// - Payment recipient (when sending to others)
	// - Sender themselves (for change outputs when spending)
	// - Anyone receiving funds in any transaction
}

// TxInput represents a reference to a previous output being spent
// Think of it as a "key" that tries to open a previous output's "lock"
// It contains proof (signature) that the spender owns the referenced output
type TxInput struct {
	ID        []byte // Transaction ID containing the output being spent
	Out       int    // Index of the output in that transaction
	Signature []byte // Digital signature proving ownership of the output
	PubKey    []byte // Full public key of the spender (not hashed, used for verification)
}

// NewTXOutput creates a new transaction output locked to an address
// This is a convenience constructor for creating outputs in transactions
// Can be used for:
// - Payments to others (value sent to recipient)
// - Change back to sender (remaining value after payment)
// - Any output creation in a transaction
func NewTXOutput(value int, address string) *TxOutput {
	// Create output with nil PubKeyHash initially
	txo := &TxOutput{value, nil}

	// Lock it to the specified address
	// The Address can be recipient's OR sender's (for change)
	txo.Lock([]byte(address))

	return txo
}

// UsesKey checks if this input was created/signed with a specific public key hash
// This verifies whether the input is spending an output that belongs to the given address
// Used to determine which inputs belong to a wallet when calculating balance
func (in *TxInput) UsesKey(pubKeyHash []byte) bool {
	// Hash the input's public key (full key) to get its hash representation
	// This simulates how the original output was locked
	lockingHash := wallet.PublicKeyHash(in.PubKey)

	// Compare with the target hash to see if they match
	// If true: This input was signed by the owner of pubKeyHash
	// Meaning: The input is spending an output that belongs to this address
	return bytes.Compare(lockingHash, pubKeyHash) == 0
}

// Lock "locks" an output to a specific Base58-encoded address
// This means only the owner of that address can spend this output later
// IMPORTANT: The address must decode to exactly 25 bytes (version + hash + checksum)
func (out *TxOutput) Lock(address []byte) {
	// Decode the Base58 address to get the full 25-byte encoded data
	// Structure: [1 byte version] + [20 byte pubKeyHash] + [4 byte checksum]
	// Example: "1A1zP1e..." → 25 bytes of binary data
	pubKeyHash := wallet.Base58Decode(address)

	// Extract just the 20-byte public key hash (remove version and checksum)
	// Version identifies network (e.g., 0x00 for Bitcoin mainnet)
	// Checksum is for error detection, not needed for locking logic
	// Bytes 1-21 contain the actual public key hash that determines ownership
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]

	// Store the hash in the output - this output is now "locked" to that address
	// Only someone with the matching private key can create a valid signature to spend this
	out.PubKeyHash = pubKeyHash
}

// IsLockedWithKey checks if this output is locked to a specific public key hash
// This determines whether the owner of the given public key hash can spend this output
// Used when:
// - Verifying if a signature in an input matches this output's lock
// - Finding which outputs belong to a wallet (by checking against wallet's pubKeyHash)
func (out *TxOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	// Direct comparison: does the provided hash match this output's lock?
	// If true: The owner of pubKeyHash can spend this output
	// This works for both payment outputs AND change outputs
	return bytes.Compare(out.PubKeyHash, pubKeyHash) == 0
}
