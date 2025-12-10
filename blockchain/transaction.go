package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/golang-blockchain/wallet"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 03/12/2025
 * Time: 12:22
 */

/*
   Transactions are composed of inputs and outputs rather than containing direct data.
   This structure prevents tampering by requiring verification of each input's legitimacy.
   Validation is performed using the sender's public key to confirm transaction integrity.

	TRANSACTION STRUCTURE:
		Instead of storing raw data directly, transactions are built from inputs and outputs.

		WHY THIS MATTERS:
		- Inputs reference previous transaction outputs (proving funds exist)
		- Outputs define where the value goes and any change returned
		- This chain of references makes tampering practically impossible

		VERIFICATION:
		Each input must be cryptographically signed with the sender's private key.
		The network verifies these signatures using the sender's public key,
		ensuring only legitimate owners can spend their funds.

	 TRANSACTION MODEL: INPUTS → OUTPUTS

		Unlike traditional databases where we modify account balances directly,
		blockchain transactions work via an Unspent Transaction Output (UTXO) model:

		- INPUTS: References to previous transaction outputs being spent
		- OUTPUTS: New allocations of value to recipient addresses

		SECURITY IMPLICATIONS:
		1. Immutability: Altering any past transaction would break all later references
		2. Verification: Each input must have a valid digital signature
		3. Transparency: Anyone can trace funds through the input-output chain

		The sender's public key verifies their signature, proving they own the inputs.

		Transaction = Inputs (what's being spent) + Outputs (where it's going)
        This indirect structure secures the ledger against tampering.
        Validity is verified using the sender's public key on signed inputs.
*/

// Transaction represents a single transaction in the blockchain
// It contains a unique ID and references to inputs and outputs
type Transaction struct {
	ID      []byte     // Unique identifier (hash) of this transaction
	Inputs  []TxInput  // List of inputs being spent
	Outputs []TxOutput // List of outputs being created
}

// Hash generates a cryptographic hash (SHA-256) of the transaction
// This hash serves as the transaction's unique identifier (like a digital fingerprint)
// CRITICAL: The hash must NOT include the existing ID field (circular dependency)
// Used for:
// - Transaction ID generation (tx.ID = tx.Hash())
// - Digital signatures (we sign the hash, not the entire transaction)
// - Merkle tree construction in blocks
// - Preventing transaction tampering
func (tx *Transaction) Hash() []byte {
	var hash [32]byte // SHA-256 produces 32-byte hashes

	// Create a copy of the transaction to avoid modifying the original
	// This is important because we need to clear the ID for hashing
	txCopy := *tx // Shallow copy (but sufficient since we only modify ID)

	// MUST clear the ID field before hashing!
	// Why? Because the ID IS the hash of the transaction
	// Can't compute hash of something that contains the hash itself
	txCopy.ID = []byte{} // Empty slice, not nil (nil might serialize differently)

	// Step 1: Serialize the transaction copy (without ID)
	serialized := txCopy.Serialize()

	// Step 2: Compute SHA-256 hash of the serialized data
	// SHA-256 is a cryptographic hash function that produces a fixed 32-byte output
	// Properties: deterministic, fast to compute, infeasible to reverse, small changes produce completely different hashes
	hash = sha256.Sum256(serialized)

	// Return as a slice (not fixed array) for flexibility
	// This 32-byte hash becomes the transaction's unique ID
	return hash[:]
}

// Serialize converts the entire transaction into a binary byte array
// This is essential for:
// - Storing transactions in blocks/persistence
// - Hashing (digital fingerprint generation)
// - Network transmission between nodes
// - Database storage
// Uses Go's built-in gob (Go Binary) encoding for serialization
func (tx Transaction) Serialize() []byte {
	var encoded bytes.Buffer // Buffer to hold the serialized data

	// Create a new gob encoder that writes to our buffer
	// gob is Go's own binary serialization format (similar to Protocol Buffers)
	enc := gob.NewEncoder(&encoded)

	// Encode the entire transaction struct into binary format
	// This includes: ID, Inputs (each with ID, Out, Signature, PubKey), Outputs (each with Value, PubKeyHash)
	err := enc.Encode(tx)
	Handle(err) // In production, you'd return the error instead of panicking

	// Return the serialized bytes
	// Example output: [binary data representing the transaction]
	return encoded.Bytes()
}

// SetID generates a unique hash identifier for the transaction
// This hash serves as the transaction's fingerprint and ensures data integrity
func (tx *Transaction) SetID() {
	var encoded bytes.Buffer // Buffer to hold encoded transaction data
	var hash [32]byte        // Array to store the SHA-256 hash

	// Step 1: Encode the entire transaction into a binary format
	encode := gob.NewEncoder(&encoded)
	err := encode.Encode(tx) // Serialize transaction struct to bytes
	Handle(err)              // Handle any encoding errors

	// Step 2: Create SHA-256 hash of the encoded data
	hash = sha256.Sum256(encoded.Bytes())

	// Step 3: Assign the hash as the transaction ID
	// Convert a fixed-size array to slice for flexibility
	tx.ID = hash[:]
}

// CoinbaseTx creates the special "mining reward" transaction
// This is the first transaction in each block, creating new coins from nothing
func CoinbaseTx(to, data string) *Transaction {
	// If no custom data provided, use a default mining message
	if data == "" {
		data = fmt.Sprintf("Coin to: %s", to)
	}

	// Coinbase inputs are special - they reference "nothing" (no previous output)
	// ID: empty (no previous transaction)
	// Out: -1 (invalid index, indicating no specific output)
	// Signature: contains arbitrary data (often mining pool name or miner's message)
	txIN := TxInput{[]byte{}, -1, nil, []byte(data)}

	// Coinbase creates new coins as output
	// Value: reward amount (100 tokens in this example)
	// PubKey: recipient's address who can spend these coins
	txOUT := NewTXOutput(100, to)

	// Create the transaction with no ID initially
	tx := Transaction{nil, []TxInput{txIN}, []TxOutput{*txOUT}}

	// Generate the transaction ID (hash of its contents)
	tx.SetID()

	return &tx
}

// IsCoinbase checks if a transaction is a coinbase (mining reward) transaction
// Coinbase transactions have special properties that distinguish them from regular transfers
func (tx *Transaction) IsCoinbase() bool {
	// A coinbase transaction must have exactly one input
	// That input must reference "nothing" (empty ID and -1 output index)
	return len(tx.Inputs) == 1 &&
		len(tx.Inputs[0].ID) == 0 &&
		tx.Inputs[0].Out == -1
}

// Sign signs all inputs of a transaction using the provided private key
// This proves the signer owns the outputs being spent
// Coinbase transactions (mining rewards) are not signed
func (tx *Transaction) Sign(privateKey ecdsa.PrivateKey, prevTXs map[string]Transaction) {
	// Coinbase transactions create new coins, they don't spend existing outputs
	// Therefore, they don't need signatures
	if tx.IsCoinbase() {
		return
	}

	// Validate that all previous transactions referenced by inputs exist
	// This prevents signing transactions that reference non-existent outputs
	for _, in := range tx.Inputs {
		prevTxID := hex.EncodeToString(in.ID)
		if prevTXs[prevTxID].ID == nil {
			log.Panic("ERROR: Previous transaction not found!")
		}
	}

	// Create a trimmed copy of the transaction for signing
	// The copy excludes signatures because we're creating them now
	// It also excludes public keys as they'll be temporarily set for each input
	txCopy := tx.TrimmedCopy()

	// Sign each input individually
	// Each input references a different previous output that needs separate proof
	for inID, in := range txCopy.Inputs {
		// Get the previous transaction that created the output this input is spending
		prevTxID := hex.EncodeToString(in.ID)
		prevTX := prevTXs[prevTxID]

		// Clear any existing signature in the copy (should already be nil from TrimmedCopy)
		txCopy.Inputs[inID].Signature = nil

		// Temporarily set the public key to the hash of the previous output's lock
		// This connects: "We're signing a transaction that spends an output locked to this hash"
		txCopy.Inputs[inID].PubKey = prevTX.Outputs[in.Out].PubKeyHash

		// Generate the transaction hash that will be signed
		// This hash includes the modified public key field, linking signature to specific output
		txCopy.ID = txCopy.Hash()

		// Clear the public key field after hashing
		// This ensures the signature is tied to the specific output, not the key itself
		txCopy.Inputs[inID].PubKey = nil

		// Create the digital signature using the private key
		// ECDSA produces two values: r and s
		r, s, err := ecdsa.Sign(rand.Reader, &privateKey, txCopy.ID)
		Handle(err)

		// Combine r and s into a single signature (standard practice: r || s)
		signature := append(r.Bytes(), s.Bytes()...)

		// Store the signature in the ORIGINAL transaction (not the copy)
		tx.Inputs[inID].Signature = signature
	}
}

// Verify checks if all signatures in the transaction are valid
// Returns true only if ALL inputs have valid signatures
// Coinbase transactions are always valid (no signatures to verify)
func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	// Coinbase transactions (mining rewards) are always valid as they create new coins
	// They don't spend existing outputs, so no signatures to verify
	if tx.IsCoinbase() {
		return true
	}

	// First, verify all referenced previous transactions exist
	// We need these to know what outputs are being spent and their locking conditions
	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("Previous transaction not correct")
		}
	}

	// Create a trimmed copy of the transaction for verification
	// This copy has nil Signature and PubKey fields, matching the state during signing
	txCopy := tx.TrimmedCopy()

	// Define the elliptic curve used for ECDSA cryptography (must match signing curve)
	curve := elliptic.P256()

	// Verify each input's signature individually
	// ALL signatures must pass verification for the transaction to be valid
	for inId, in := range tx.Inputs {
		// Get the previous transaction that created the output this input is spending
		prevTx := prevTXs[hex.EncodeToString(in.ID)]

		// Prepare the transaction copy exactly as it was during signing:
		// 1. Clear the signature field (should already be nil from TrimmedCopy)
		txCopy.Inputs[inId].Signature = nil

		// 2. Temporarily set the public key to the hash of the previous output being spent
		// This links the signature verification to the specific output
		txCopy.Inputs[inId].PubKey = prevTx.Outputs[in.Out].PubKeyHash

		// 3. Compute the hash of the modified transaction copy
		// This should produce the EXACT same hash that was signed originally
		txCopy.ID = txCopy.Hash()

		// 4. Clear the public key field after hashing
		// The signature is tied to the transaction hash, not the key itself
		txCopy.Inputs[inId].PubKey = nil

		// Extract the r and s components from the ECDSA signature
		// ECDSA signatures consist of two 256-bit integers (r, s)
		r := big.Int{}
		s := big.Int{}

		// Get the signature length and split it into r and s components
		// Signature format: first half = r, second half = s
		sigLen := len(in.Signature)
		r.SetBytes(in.Signature[:(sigLen / 2)])
		s.SetBytes(in.Signature[(sigLen / 2):])

		// Extract the X and Y coordinates from the public key,
		// Public key format for P-256: X (32 bytes) concatenated with Y (32 bytes)
		x := big.Int{}
		y := big.Int{}

		// Get the public key length and split into X and Y coordinates
		keyLen := len(in.PubKey)
		x.SetBytes(in.PubKey[:(keyLen / 2)])
		y.SetBytes(in.PubKey[(keyLen / 2):])

		// Reconstruct the ECDSA public key object from the extracted coordinates
		rawPubKey := ecdsa.PublicKey{
			Curve: curve, // P-256 elliptic curve
			X:     &x,    // X coordinate on the curve
			Y:     &y,    // Y coordinate on the curve
		}

		// Verify the digital signature using the public key
		// This checks: "Was this transaction hash signed by the private key corresponding to this public key?"
		if ecdsa.Verify(&rawPubKey, txCopy.ID, &r, &s) == false {
			return false // Signature verification failed for this input
		}
	}

	// All input signatures verified successfully
	return true
}

// NewTransaction creates a new transaction transferring tokens from one address to another
// This is the main transaction constructor that builds valid, spendable transactions
// by selecting inputs, creating outputs, and calculating change.
func NewTransaction(from, to string, amount int, UTXO *UTXOSet) *Transaction {
	// Step 1: Initialize empty input and output collections
	var inputs []TxInput   // Will reference outputs being spent
	var outputs []TxOutput // Will define where funds go

	// Step 2: Load wallets and get sender's wallet
	wallets, err := wallet.CreateWallets()
	Handle(err)

	// Get the sender's wallet by address
	w := wallets.GetWallet(from)

	// Validate wallet exists
	if w == nil {
		log.Panic("Wallet not found for address: " + from)
	}

	// Get the sender's public key hash (this identifies which outputs they own)
	pubKeyHash := wallet.PublicKeyHash(w.PublicKey)

	// Find enough unspent outputs owned by the sender to cover the requested amount
	// Returns: total value found, and which specific outputs to spend
	acc, validOutputs := UTXO.FindSpendableOutputs(pubKeyHash, amount)

	// Step 3: Validate sufficient funds before proceeding
	if acc < amount {
		log.Panic("Error: Not enough funds!")
	}

	// Step 4: Convert selected outputs into transaction inputs
	// Each UTXO being spent becomes an input in the new transaction
	for id, outs := range validOutputs {
		// Convert hex transaction ID string back to bytes
		txID, err := hex.DecodeString(id)
		Handle(err)

		// Create an input for each selected output
		for _, out := range outs {
			input := TxInput{
				ID:        txID,        // Previous transaction ID
				Out:       out,         // Which output in that transaction
				Signature: nil,         // Will be set after signing
				PubKey:    w.PublicKey, // Sender's public key (for verification)
			}
			inputs = append(inputs, input)
		}
	}

	// Step 5: Create the payment output to the recipient
	outputs = append(outputs, *NewTXOutput(amount, to))

	// Step 6: Create change output back to sender (if needed)
	if acc > amount {
		change := acc - amount
		outputs = append(outputs, *NewTXOutput(change, from))
	}

	// Step 7: Construct the transaction
	tx := Transaction{
		ID:      nil,     // Will be set to hash
		Inputs:  inputs,  // Inputs spending UTXOs
		Outputs: outputs, // New outputs being created
	}

	// Step 8: Generate transaction ID (hash)
	// Must be done BEFORE signing because signatures sign the hash
	tx.SetID() // Sets tx.ID = tx.Hash()

	// Step 9: Sign the transaction with the sender's private key
	// This creates digital signatures proving ownership of inputs
	UTXO.Blockchain.SignTransaction(&tx, w.PrivateKey)

	// Step 10: Return the completed, signed transaction
	return &tx
}

// TrimmedCopy creates a modified copy of the transaction for signing/verification
// It removes all signatures and public keys from inputs while preserving everything else
// This is CRITICAL because:
// 1. You can't sign something that includes the signature you're about to create
// 2. Public keys are set temporarily during signing, then cleared after hashing
// 3. This creates a deterministic transaction representation for hashing
func (tx *Transaction) TrimmedCopy() Transaction {
	var inputs []TxInput   // New slice for modified inputs
	var outputs []TxOutput // New slice for outputs (unchanged)

	// Copy each input but with empty signature and public key fields
	// This ensures the hash we sign doesn't include existing signatures or keys
	for _, in := range tx.Inputs {
		inputs = append(inputs, TxInput{
			ID:        in.ID,  // Keep reference of the previous transaction
			Out:       in.Out, // Keep which output index is being spent
			Signature: nil,    // EMPTY: Will be filled during signing
			PubKey:    nil,    // EMPTY: Will be temporarily set during signing
		})
	}

	// Copy outputs exactly as they are
	// Outputs don't affect signing of inputs (they're what's being created, not spent)
	for _, out := range tx.Outputs {
		outputs = append(outputs, TxOutput{
			Value:      out.Value,      // Amount being sent
			PubKeyHash: out.PubKeyHash, // Hash of recipient's key (the "lock")
		})
	}

	// Create and return the trimmed transaction copy
	// Note: We keep the original transaction ID in the copy
	// The ID will be recalculated during signing/verification
	txCopy := Transaction{
		ID:      tx.ID,   // Original ID (will be ignored during hashing)
		Inputs:  inputs,  // Inputs with nil Signature/PubKey
		Outputs: outputs, // Unchanged outputs
	}
	return txCopy
}

// String returns a human-readable representation of the transaction
// Useful for debugging, logging, and displaying transaction details
func (tx Transaction) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("--- Transaction %x:", tx.ID))
	for i, in := range tx.Inputs {
		lines = append(lines, fmt.Sprintf("			Input %d:", i))
		lines = append(lines, fmt.Sprintf("			Previous TxID: %x", in.ID))
		lines = append(lines, fmt.Sprintf("			Output Index: %d", in.Out))
		lines = append(lines, fmt.Sprintf("			Signature: %x", in.Signature))
		lines = append(lines, fmt.Sprintf("			Pubkey: %x", in.PubKey))
	}

	for i, output := range tx.Outputs {
		lines = append(lines, fmt.Sprintf("			Output %d:", i))
		lines = append(lines, fmt.Sprintf("			Value: %d", output.Value))
		lines = append(lines, fmt.Sprintf("			Script: %x", output.PubKeyHash)) // Script is used to derive the address
	}

	return strings.Join(lines, "\n")
}
