package blockchain

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 29/11/2025
 * Time: 14:21
 */

import (
	"bytes"
	"crypto/ecdsa"
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

	opts := badger.DefaultOptions(dbPath).WithLogger(nil)
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
func (chain *Blockchain) AddBlock(transactions []*Transaction) *Block {
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

	return newBlock
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

// FindUTXO scans the entire blockchain to build a complete map of all unspent transaction outputs
// This is used to initialize or rebuild the UTXO set index
// Returns: map[TransactionID] -> TxOutputs (collection of unspent outputs for that transaction)
func (chain *Blockchain) FindUTXO() map[string]TxOutputs {
	// UTXO map: TransactionID (hex string) -> All unspent outputs from that transaction
	UTXO := make(map[string]TxOutputs)

	// Track which outputs have been spent
	// Key: TransactionID (hex string)
	// Value: List of output indices that have been spent in that transaction
	spentTXOs := make(map[string][]int)

	// Create iterator to traverse blockchain from newest to oldest
	// Reverse traversal is CRITICAL: we need to see spending transactions
	// before we see the outputs they're spending
	iter := chain.Iterator()

	// Process blocks in reverse chronological order (newest first)
	for {
		block := iter.Next() // Get the next block (starting from latest)

		// Process all transactions in the current block
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID) // Convert transaction ID to a string key

			// Check each OUTPUT in this transaction
			// Outputs create new UTXOs (unless they're spent later)
		Outputs:
			for outIdx, out := range tx.Outputs {
				// Check if any outputs from this transaction have been marked as spent
				if spentTXOs[txID] != nil {
					// Verify THIS specific output index hasn't been spent
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							// This output has been spent! Skip to the next output
							continue Outputs
						}
					}
				}

				// If we reach here, this output is NOT spent (it's a UTXO!)
				// Add it to our UTXO map for this transaction
				outs := UTXO[txID]                       // Get existing outputs for this transaction
				outs.Outputs = append(outs.Outputs, out) // Add this unspent output
				UTXO[txID] = outs                        // Update map
			}

			// Check each INPUT in this transaction (if not coinbase)
			// Inputs SPEND previous outputs, marking them as no longer UTXOs
			if tx.IsCoinbase() == false {
				for _, in := range tx.Inputs {
					inTxID := hex.EncodeToString(in.ID) // Transaction being spent FROM
					outIndex := in.Out                  // Which output index is being spent?

					// Mark this output as spent
					// Example: spentTXOs["abc123"] = [0, 2] means outputs 0 and 2 of transaction "abc123" are spent
					spentTXOs[inTxID] = append(spentTXOs[inTxID], outIndex)
				}
			}
		}

		// Check if we've reached the genesis block (beginning of a chain)
		if len(block.PrevHash) == 0 {
			break // Stop iteration at genesis block
		}
	}

	// Return a complete UTXO map
	// Each entry contains only the UNSPENT outputs for that transaction
	return UTXO
}

// FindTransaction searches the entire blockchain for a specific transaction by ID
// Returns the transaction if found, or an error if not found
func (bc *Blockchain) FindTransaction(ID []byte) (Transaction, error) {
	// Create an iterator to traverse blocks from newest to oldest
	// This is more efficient than iterating from genesis when looking for recent transactions
	iter := bc.Iterator()

	// Loop through all blocks in the chain
	for {
		// Get the next block (starts with the latest block)
		block := iter.Next()

		// Search all transactions in the current block
		for _, tx := range block.Transactions {
			// Compare transaction ID with target ID
			// bytes.Compare returns 0 if the byte slices are equal
			if bytes.Compare(tx.ID, ID) == 0 {
				// Transaction found! Return a copy of it
				return *tx, nil
			}
		}

		// Check if we've reached the genesis block (start of a chain)
		// Genesis block has empty PreviousHash (length 0)
		if len(block.PrevHash) == 0 {
			break // Stop searching, reached the beginning of a chain
		}
	}

	// If we get here, the transaction was not found in any block
	return Transaction{}, fmt.Errorf("transaction does not exist")
}

// SignTransaction signs a transaction by finding all referenced previous transactions
// and calling the transaction's Sign method with the private key
func (bc *Blockchain) SignTransaction(tx *Transaction, privateKey ecdsa.PrivateKey) {
	// Create a map to store previous transactions referenced by this transaction's inputs
	// Key: Previous transaction ID (as hex string)
	// Value: The actual Transaction object
	prevTXs := make(map[string]Transaction)

	// For each input in the transaction, find the transaction it's spending from
	for _, in := range tx.Inputs {
		// Find the transaction that created the output this input is trying to spend
		prevTX, err := bc.FindTransaction(in.ID)
		Handle(err) // Panic if transaction not found (shouldn't happen in valid blockchain)

		// Store it in the map using hex-encoded ID as a key
		prevTXs[hex.EncodeToString(in.ID)] = prevTX
	}

	// Now that we have all previous transactions, sign the current transaction;
	// The transaction's Sign method will:
	// 1. Create a trimmed copy (without signatures)
	// 2. For each input, set appropriate fields
	// 3. Hash the transaction
	// 4. Create digital signatures
	// 5. Store signatures back in the original transaction
	tx.Sign(privateKey, prevTXs)
}

// VerifyTransaction checks if a transaction's signatures are valid
// This is crucial for preventing unauthorized spending
func (bc *Blockchain) VerifyTransaction(tx *Transaction) bool {
	// Coinbase transactions (mining rewards) don't need verification
	// They create new coins, not spend existing ones
	if tx.IsCoinbase() {
		return true
	}

	// Create a map for previous transactions (same as in SignTransaction)
	prevTXs := make(map[string]Transaction)

	// Find all transactions that are being spent by this transaction's inputs
	for _, in := range tx.Inputs {
		prevTX, err := bc.FindTransaction(in.ID)
		Handle(err) // If we can't find the transaction, it's invalid

		// Store the previous transaction
		prevTXs[hex.EncodeToString(in.ID)] = prevTX
	}

	// Verify all signatures in the transaction
	// The Verify method will:
	// 1. Create a trimmed copy (without signatures)
	// 2. For each input, reconstruct what was signed
	// 3. Verify the digital signature using the public key
	// 4. Return true only if ALL signatures are valid
	return tx.Verify(prevTXs)
}
