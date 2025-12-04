package blockchain

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 29/11/2025
 * Time: 14:21
 */

import (
	"encoding/hex"
	"fmt"
	"os"
	"runtime"

	"github.com/dgraph-io/badger/v4"
)

const (
	dbPath      = "./tmp/blocks"
	dbFile      = "./tmp/blocks/MANIFEST"
	genesisData = "Fist Transaction From Genesis"
)

type Blockchain struct {
	LastHash []byte     // The hash of the last block in the blockchain
	Database *badger.DB // The database for storing the blockchain
}

// Iterator Struct for iterating through the blockchain in a badger database
type Iterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

// DBExists Special function for checking if the database file exists
func DBExists() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}
	return true
}

// InitBlockChain Address passed below is the address of the miner which gets the reward for mining the first block
func InitBlockChain(address string) *Blockchain {
	if DBExists() {
		fmt.Println("Blockchain already exists!")
		runtime.Goexit()
	}

	var lastHash []byte

	opts := badger.DefaultOptions(dbPath)
	opts.Dir = dbPath      // Key and metadata will be stored in this directory
	opts.ValueDir = dbPath // Value will be stored in this directory

	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		cbTXN := CoinbaseTx(address, genesisData)
		genesis := Genesis(cbTXN)
		fmt.Println("Genesis block created")
		err = txn.Set(genesis.Hash, genesis.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), genesis.Hash)
		lastHash = genesis.Hash
		return err
	})
	Handle(err)

	chain := Blockchain{lastHash, db}
	return &chain
}

func ContinueBlockChain(address string) *Blockchain {
	if DBExists() == false {
		fmt.Println("No existing blockchain found, create a one!")
		runtime.Goexit()
	}

	var lastHash []byte
	opts := badger.DefaultOptions(dbPath)
	opts.Dir = dbPath      // Key and metadata will be stored in this directory
	opts.ValueDir = dbPath // Value will be stored in this directory

	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		err = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})
		return err
	})
	Handle(err)

	chain := Blockchain{lastHash, db}
	return &chain
}

// AddBlock Special function for adding the block to the blockchain
func (chain *Blockchain) AddBlock(transactions []*Transaction) {
	var lastHash []byte

	// Read-only type of database transaction for getting the last hash value
	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		err = item.Value(func(val []byte) error {
			lastHash = val
			return nil
		})
		return err

	})
	Handle(err)

	newBlock := CreateBlock(transactions, lastHash)

	// Read-write type of transaction for adding the new block to the blockchain
	err = chain.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		Handle(err)
		// Storing the last hash value
		err = txn.Set([]byte("lh"), newBlock.Hash)
		// Update the last hash value for the next block
		chain.LastHash = newBlock.Hash
		return err
	})
}

// Iterator Special function for creating an iterator for iterating through the blockchain
func (chain *Blockchain) Iterator() *Iterator {
	iterator := &Iterator{chain.LastHash, chain.Database}
	return iterator
}

func (iter *Iterator) Next() *Block {
	var block *Block
	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		Handle(err)
		return item.Value(func(val []byte) error {
			block = Deserialize(val)
			return nil
		})
	})
	Handle(err)

	iter.CurrentHash = block.PrevHash // Since it is going backward until genesis block
	return block
}

// FindUnspentTransactions returns all transactions containing UTXOs (unspent outputs)
// that belong to the specified address. This is essential for:
// 1. Calculating an address's balance (sum of all UTXO values)
// 2. Finding spendable inputs for new transactions
// 3. Preventing double-spending by tracking which outputs have been consumed
func (chain *Blockchain) FindUnspentTransactions(address string) []Transaction {
	// Store all transactions that contain unspent outputs for the address
	var unspentTXNs []Transaction

	// Track outputs that have been spent to avoid double-counting
	// Key: Transaction ID (hex string)
	// Value: List of output indices that have been spent within that transaction
	spentTXNs := make(map[string][]int)

	// Create an iterator to traverse the blockchain from newest to oldest block
	// We MUST iterate backward (newest to oldest) to correctly identify spent outputs
	iter := chain.Iterator()

	// Begin iterating through blocks in reverse chronological order
	// Reverse traversal is CRITICAL: we need to see spending transactions
	// before we see the outputs they're spending
	for {
		block := iter.Next() // Get the next block (starting from latest)

		// Process all transactions in the current block
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID) // Convert transaction ID to string for a map key

			// ============================================================
			// STEP 1: Check each OUTPUT in this transaction
			// ============================================================
			// We examine outputs to find ones that:
			// 1. Are owned by our address AND
			// 2. Haven't been spent yet
		Outputs: // Label for control flow (skip to the next output if already spent)
			for outIDx, out := range tx.Outputs {
				// Check if this transaction has any spent outputs recorded
				if spentTXNs[txID] != nil {
					// Check if THIS specific output index has been marked as spent
					for _, spentOut := range spentTXNs[txID] {
						if spentOut == outIDx {
							// Output has been spent! Skip to the next output
							continue Outputs
						}
					}
				}

				// If we reach here, output is not spent
				// Check if this output is owned by our address
				if out.CanBeUnlocked(address) {
					// Found an unspent output owned by our address!
					// Add the entire transaction to the result (transaction may have multiple outputs)
					unspentTXNs = append(unspentTXNs, *tx)
				}
			}

			// ============================================================
			// STEP 2: Check each INPUT in this transaction (if not coinbase)
			// ============================================================
			// Coinbase transactions create new coins (no inputs to check)
			if tx.IsCoinbase() == false {
				for _, in := range tx.Inputs {
					// Check if this input is spending funds FROM our address
					if in.CanUnlock(address) {
						// This input spends an output that belongs to our address
						// We need to mark that output as spent so we don't count it later

						inTxID := hex.EncodeToString(in.ID) // Get the transaction being spent FROM
						outIndex := in.Out                  // Get which output index is being spent

						// Record that this specific output is now spent
						// Example: spentTXNs["abc123"] = [0, 2] means outputs 0 and 2 of tx "abc123" are spent
						spentTXNs[inTxID] = append(spentTXNs[inTxID], outIndex)
					}
				}
			}
		}

		// Check if we've reached the genesis block (beginning of a chain)
		if len(block.PrevHash) == 0 {
			break // Stop iteration at genesis block
		}
	}

	// Return all transactions that contain unspent outputs for the address
	return unspentTXNs
}

// FindUTXO returns all Unspent Transaction Outputs (UTXOs) for a given address
//
// UTXOs are the fundamental building blocks for spending in UTXO-based blockchains.
// This method provides a filtered view of only the spendable outputs,
// which is essential for wallet operations and transaction construction.
func (chain *Blockchain) FindUTXO(address string) []TxOutput {
	// Initialize slice to collect all UTXOs owned by the address
	var UTXOs []TxOutput

	// Step 1: Get all transactions containing outputs for this address
	// This includes transactions that may have both spendable and already-spent outputs
	// FindUnspentTransactions returns full transactions, not individual outputs
	unspentTXNs := chain.FindUnspentTransactions(address)

	// Step 2: Filter each transaction to extract only the outputs owned by the address
	// This is necessary because a transaction can have multiple outputs to different addresses
	for _, tx := range unspentTXNs {
		// Examine each output in the transaction
		for _, out := range tx.Outputs {
			// Check if this specific output is locked to our address
			// This ensures we only include outputs we can actually spend
			if out.CanBeUnlocked(address) {
				// Add this spendable output to our UTXO collection
				UTXOs = append(UTXOs, out)
			}
		}
	}

	// Return the complete list of spendable outputs
	// Each UTXO in this list:
	// 1. Belongs to the specified address
	// 2. Has not been spent in any later transaction
	// 3. Can be used as input for a new transaction
	return UTXOs
}

// FindSpendableOutputs selects UTXOs to cover a payment amount and returns them
// in a format ready for transaction construction.
//
// This is the core "coin selection" algorithm for UTXO-based blockchains.
// It finds and groups enough unspent outputs to meet or exceed the requested amount.
func (chain *Blockchain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	// unspentOuts maps TransactionID -> []OutputIndices
	// This structure is optimized for creating transaction inputs:
	// Example: {"abc123": [0, 2]} means spend outputs 0 and 2 from transaction abc123
	unspentOuts := make(map[string][]int)

	// Step 1: Get all transactions containing potential spendable outputs
	// Note: These transactions may contain outputs for other addresses too
	unspentTxs := chain.FindUnspentTransactions(address)

	// Track the total value accumulated from selected outputs
	// We keep adding outputs until we have enough to cover the payment
	accumulated := 0

	// Label for breaking out of nested loops when we have enough funds
Work:
	// Step 2: Iterate through candidate transactions (order matters for coin selection)
	for _, tx := range unspentTxs {
		// Convert transaction ID to string for use as a map key
		txID := hex.EncodeToString(tx.ID)

		// Step 3: Check each output in this transaction
		for outIdx, out := range tx.Outputs {
			// Three conditions must be met:
			// 1. Output belongs to our address (CanBeUnlocked)
			// 2. We haven't reached the target amount yet (accumulated < amount)
			// 3. In real implementations: Also check output isn't already selected

			if out.CanBeUnlocked(address) && accumulated < amount {
				// This output qualifies! Add it to our selection
				accumulated += out.Value
				unspentOuts[txID] = append(unspentOuts[txID], outIdx)

				// Step 4: Check if we've collected enough value
				// Note: We collect AT LEAST the requested amount (may get more due to indivisible outputs)
				if accumulated >= amount {
					// We have enough! Exit both loops using the Work label
					break Work
				}
				// Otherwise, continue to next output
			}
		}
	}

	// Return both:
	// 1. The total accumulated value (maybe more than the requested amount)
	// 2. The selected outputs are grouped by transaction for easy input creation
	return accumulated, unspentOuts
}
