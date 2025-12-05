package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
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
	txIN := TxInput{[]byte{}, -1, data}

	// Coinbase creates new coins as output
	// Value: reward amount (100 tokens in this example)
	// PubKey: recipient's address who can spend these coins
	txOUT := TxOutput{100, to}

	// Create the transaction with no ID initially
	tx := Transaction{nil, []TxInput{txIN}, []TxOutput{txOUT}}

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

// NewTransaction creates a new transaction transferring tokens from one address to another
// This is the main transaction constructor that builds valid, spendable transactions
// by selecting inputs, creating outputs, and calculating change.
func NewTransaction(from, to string, amount int, chain *Blockchain) *Transaction {
	// Step 1: Initialize empty input and output collections
	var inputs []TxInput   // Will reference outputs being spent
	var outputs []TxOutput // Will define where funds go

	// Step 2: Find enough spendable outputs to cover the requested amount
	// acc: Total value accumulated from selected outputs (may exceed amount)
	// validOutputs: Map of TransactionID -> OutputIndices selected for spending
	acc, validOutputs := chain.FindSpendableOutputs(from, amount)

	// Step 3: Validate sufficient funds before proceeding
	if acc < amount {
		// In real implementation, this would return an error, not panic
		log.Panic("Error: Not enough funds!")
	}

	// Step 4: Convert selected outputs into transaction inputs
	// Each output selected in FindSpendableOutputs becomes an input in this transaction
	for id, outs := range validOutputs {
		// Convert hex string transaction ID back to byte slice
		txID, err := hex.DecodeString(id)
		Handle(err)

		// For each output index selected from this transaction
		for _, out := range outs {
			// Create an input that references the output being spent
			// Note: 'from' as Signature is simplified - a real version would have actual cryptographic signature
			input := TxInput{
				ID:        txID, // The transaction containing the output we're spending
				Out:       out,  // Which output index within that transaction
				Signature: from, // Simplified: address as "signature" (real: actual digital signature)
			}
			inputs = append(inputs, input)
		}
	}

	// Step 5: Create the primary output - payment to recipient
	// This output sends the requested amount to the destination address
	outputs = append(outputs, TxOutput{
		Value:  amount, // Amount being sent
		PubKey: to,     // Recipient's address (who can spend this output later)
	})

	// Step 6: Handle change - if we selected more value than needed,
	// UTXOs are indivisible, so we often need to create "change" back to ourselves
	if acc > amount {
		// Calculate change amount: The total selected minus payment amount
		change := acc - amount

		// Create change output that sends excess back to the sender
		outputs = append(outputs, TxOutput{
			Value:  change, // Change amount
			PubKey: from,   // Sender's address (they get their change back)
		})

		// Example: Selected 90 tokens total, sent 65 to recipient
		// Change output: 25 tokens back to sender
	}

	// Step 7: Construct the transaction object
	tx := Transaction{
		ID:      nil,     // ID will be set by SetID()
		Inputs:  inputs,  // References to outputs being spent
		Outputs: outputs, // New outputs being created
	}

	// Step 8: Generate transaction ID (hash of transaction contents)
	// This must be done AFTER all fields are populated
	tx.SetID()

	// Step 9: Return the completed, valid transaction
	return &tx
}
