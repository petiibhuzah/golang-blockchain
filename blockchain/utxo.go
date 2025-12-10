package blockchain

import (
	"bytes"
	"encoding/hex"
	"log"

	"github.com/dgraph-io/badger/v4"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 10/12/2025
 * Time: 11:19
 */

// UTXO (Unspent Transaction Output) Set is an optimized data structure
// that tracks all spendable outputs without scanning the entire blockchain
// This dramatically improves performance for wallet operations
var (
	utxoPrefix   = []byte("utxo-") // Database key prefix for UTXO entries
	prefixLength = len(utxoPrefix) // Length of prefix for key manipulation
)

// UTXOSet represents the collection of all unspent transaction outputs
// It's maintained as a separate index for fast lookups
type UTXOSet struct {
	Blockchain *Blockchain // Reference to the blockchain for full data access
}

// FindSpendableOutputs finds enough UTXOs to cover a payment amount
// This is the core "coin selection" algorithm for creating transactions
func (u UTXOSet) FindSpendableOutputs(pubkeyHash []byte, amount int) (int, map[string][]int) {
	// Map to store selected outputs: TransactionID -> []OutputIndices
	unspentOuts := make(map[string][]int)
	accumulated := 0 // Total value collected so far

	db := u.Blockchain.Database

	// Read-only transaction for safe concurrent access
	err := db.View(func(txn *badger.Txn) error {
		// Configure iterator to scan only UTXO entries
		opts := badger.DefaultIteratorOptions

		it := txn.NewIterator(opts)
		defer it.Close()

		// Iterate through all UTXO entries (keys starting with "utxo-")
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			k := item.Key() // Key format: "utxo-" + transactionID

			// Deserialize the outputs stored for this transaction
			outs := TxOutputs{}
			err := item.Value(func(val []byte) error {
				outs = DeserializeOutputs(val)
				return nil
			})

			// Remove the "utxo-" prefix to get the raw transaction ID
			k = bytes.TrimPrefix(k, utxoPrefix)
			txID := hex.EncodeToString(k) // Convert to string for a map key
			Handle(err)

			// Check each output in this transaction
			for outIdx, out := range outs.Outputs {
				// Two conditions must be met:
				// 1. Output belongs to our address (matching pubkeyHash)
				// 2. We haven't collected enough value yet
				if out.IsLockedWithKey(pubkeyHash) && accumulated < amount {
					// Add this output to our selection
					accumulated += out.Value
					unspentOuts[txID] = append(unspentOuts[txID], outIdx)
				}
			}
		}
		return nil
	})

	Handle(err)
	return accumulated, unspentOuts
}

// FindUnspentTransactions returns all UTXOs owned by a specific address
// Used for calculating wallet balance and listing spendable funds
func (u UTXOSet) FindUnspentTransactions(pubkeyHash []byte) []TxOutput {
	var UTXOs []TxOutput // Collection of unspent outputs

	db := u.Blockchain.Database

	// Read-only transaction
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions

		it := txn.NewIterator(opts)
		defer it.Close()

		// Scan all UTXO entries
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			outs := TxOutputs{}

			// Deserialize outputs for this transaction
			err := item.Value(func(val []byte) error {
				outs = DeserializeOutputs(val)
				return nil
			})
			Handle(err)

			// Filter outputs belonging to our address
			for _, out := range outs.Outputs {
				if out.IsLockedWithKey(pubkeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}
		}
		return nil
	})

	Handle(err)
	return UTXOs
}

// CountTransactions returns the total number of transactions with unspent outputs
// Useful for monitoring and debugging
func (u UTXOSet) CountTransactions() int {
	db := u.Blockchain.Database
	counter := 0

	// Simple count of UTXO entries
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions

		it := txn.NewIterator(opts)
		defer it.Close()

		// Count each UTXO entry
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			counter++
		}
		return nil
	})

	Handle(err)
	return counter
}

// Reindex rebuilds the entire UTXO set from scratch
// Used during:
// 1. Initial setup
// 2. Database corruption recovery
// 3. Major blockchain reorganization
func (u UTXOSet) Reindex() {
	db := u.Blockchain.Database

	// Clear existing UTXO data
	u.DeleteByPrefix(utxoPrefix)

	// Scan the entire blockchain to find current UTXOs
	UTXO := u.Blockchain.FindUTXO()

	// Write a new UTXO set to a database
	err := db.Update(func(txn *badger.Txn) error {
		for txId, outs := range UTXO {
			// Convert hex transaction ID to bytes
			key, err := hex.DecodeString(txId)
			if err != nil {
				return err
			}

			// Add the "utxo-" prefix to create a database key
			key = append(utxoPrefix, key...)

			// Store serialized outputs
			err = txn.Set(key, outs.Serialize())
			Handle(err)
		}
		return nil
	})

	Handle(err)
}

// Update modifies the UTXO set when a new block is added to the blockchain
// This is the most performance-critical function - called for every new block
func (u *UTXOSet) Update(block *Block) {
	db := u.Blockchain.Database

	err := db.Update(func(txn *badger.Txn) error {
		// Process each transaction in the new block
		for _, tx := range block.Transactions {
			// For regular transactions (not coinbase):
			// Remove outputs that were spent by this transaction's inputs
			if tx.IsCoinbase() == false {
				for _, in := range tx.Inputs {
					updateOuts := TxOutputs{}

					// Create a database key: "utxo-" + spentTransactionID
					inID := append(utxoPrefix, in.ID...)

					// Get the outputs for the transaction being spent from
					item, err := txn.Get(inID)
					Handle(err)

					outs := TxOutputs{}
					err = item.Value(func(val []byte) error {
						outs = DeserializeOutputs(val)
						return nil
					})
					Handle(err)

					// Keep all outputs EXCEPT the one being spent
					for outIdx, out := range outs.Outputs {
						if outIdx != in.Out { // Skip the spent output
							updateOuts.Outputs = append(updateOuts.Outputs, out)
						}
					}

					// If no outputs remain, delete the entire entry
					// Otherwise, update with remaining outputs
					if len(updateOuts.Outputs) == 0 {
						if err := txn.Delete(inID); err != nil {
							log.Panic(err)
						}
					} else {
						if err := txn.Set(inID, updateOuts.Serialize()); err != nil {
							log.Panic(err)
						}
					}
				}
			}

			// Add new outputs created by this transaction
			newOutputs := TxOutputs{}
			for _, out := range tx.Outputs {
				newOutputs.Outputs = append(newOutputs.Outputs, out)
			}

			// Store new outputs with a key: "utxo-" + newTransactionID
			txID := append(utxoPrefix, tx.ID...)
			if err := txn.Set(txID, newOutputs.Serialize()); err != nil {
				log.Panic(err)
			}
		}
		return nil
	})

	Handle(err)
}

// DeleteByPrefix efficiently deletes all keys with a given prefix
// Used for clearing the UTXO set during reindexing
func (u *UTXOSet) DeleteByPrefix(prefix []byte) {
	// Helper function to delete a batch of keys
	deleteKeys := func(keysForDelete [][]byte) error {
		if err := u.Blockchain.Database.Update(func(txn *badger.Txn) error {
			for _, key := range keysForDelete {
				if err := txn.Delete(key); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}

	collectSize := 100000 // Batch size for deletion (prevents memory issues)

	err := u.Blockchain.Database.View(func(txn *badger.Txn) error {
		// Configure iterator for keys only (no values to save memory)
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		keysForDelete := make([][]byte, 0, collectSize)
		keysCollected := 0

		// Collect keys in batches and delete them
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil) // Copy key to avoid reference issues
			keysForDelete = append(keysForDelete, key)
			keysCollected++

			// Delete keys when the batch is full
			if keysCollected == collectSize {
				if err := deleteKeys(keysForDelete); err != nil {
					log.Panic(err)
				}
				keysForDelete = make([][]byte, 0, collectSize) // Reset batch
				keysCollected = 0
			}
		}

		// Delete any remaining keys
		if keysCollected > 0 {
			if err := deleteKeys(keysForDelete); err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
	Handle(err)
}
