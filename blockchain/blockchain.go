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
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dgraph-io/badger/v4"
)

const (
	dbPath      = "./tmp/blocks_%s"
	genesisData = "First Transaction from Genesis"
)

type BlockChain struct {
	LastHash []byte     // The hash of the last block in the blockchain
	Database *badger.DB // The database for storing the blockchain
}

// DBExists Special function for checking if the database file exists
func DBExists(path string) bool {
	if _, err := os.Stat(path + "/MANIFEST"); os.IsNotExist(err) {
		return false
	}
	return true
}

// InitBlockChain Address passed below is the address of the miner which gets the reward for mining the first block
func InitBlockChain(address, nodeID string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeID)
	if DBExists(path) {
		fmt.Println("BlockChain already exists!")
		runtime.Goexit()
	}

	var lastHash []byte

	opts := badger.DefaultOptions(path).WithLogger(nil)
	opts.Dir = path      // Key and metadata will be stored in this directory
	opts.ValueDir = path // Value will be stored in this directory

	db, err := openDB(path, opts)
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

	chain := BlockChain{lastHash, db}
	return &chain
}

func ContinueBlockChain(nodeID string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeID)
	if DBExists(path) == false {
		fmt.Println("No existing blockchain found, create a one!")
		runtime.Goexit()
	}

	var lastHash []byte
	opts := badger.DefaultOptions(path)
	opts.Dir = path      // Key and metadata will be stored in this directory
	opts.ValueDir = path // Value will be stored in this directory

	db, err := openDB(path, opts)
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

	chain := BlockChain{lastHash, db}
	return &chain
}

// GetBestHeight returns the height (block number) of the current blockchain tip
// This function provides quick access to the current blockchain length
// Height represents how many blocks are in the chain since genesis (0-based)
func (chain *BlockChain) GetBestHeight() int {
	var lastBlock Block // Variable to store the most recent block

	// Perform a read-only database transaction to safely access the blockchain state
	err := chain.Database.View(func(txn *badger.Txn) error {
		// Step 1: Get the "last hash" pointer (key "lh")
		// This pointer always points to the hash of the current blockchain tip
		item, err := txn.Get([]byte("lh"))
		Handle(err) // Exit if you can't retrieve the tip pointer

		// Step 2: Extract the actual hash of the last block
		// ValueCopy(nil) returns a byte slice copy of the hash value
		lastHash, _ := item.ValueCopy(nil) // Note: error intentionally ignored (already handled above)

		// Step 3: Use the hash to retrieve the actual last block
		// Blocks are stored with their hash as the database key
		item, err = txn.Get(lastHash)
		Handle(err) // Exit if the last block doesn't exist (database corruption)

		// Step 4: Get the serialized block data
		lastBlockData, _ := item.ValueCopy(nil) // Get block bytes, ignore error (handled above)

		// Step 5: Deserialize bytes into Block struct
		// The asterisk (*) dereferences the pointer returned by Deserialize
		lastBlock = *Deserialize(lastBlockData)

		return nil // Transaction completed successfully
	})
	Handle(err) // Exit if any database operation failed

	// Return the height of the last block
	// Height represents the block's position in the chain (genesis = 0)
	return lastBlock.Height
}

// GetBlock retrieves a specific block from the blockchain by its hash
// This function provides direct block lookup, similar to a key-value store
// Returns the block if found, or an error if the block doesn't exist
func (chain *BlockChain) GetBlock(blockHash []byte) (Block, error) {
	var block Block // Variable to hold the retrieved block

	// Perform a read-only database transaction to safely retrieve the block
	err := chain.Database.View(func(txn *badger.Txn) error {
		// Attempt to find the block using its hash as the database key
		// In blockchain databases, blocks are typically stored with a key = block hash
		if item, err := txn.Get(blockHash); err != nil {
			// Block not found in the database
			// This could mean:
			// 1. Block doesn't exist (invalid hash)
			// 2. Block was orphaned/replaced in a chain reorganization
			// 3. Database corruption
			return errors.New("block not found")
		} else {
			// Block found - retrieve its serialized data from the database
			// ValueCopy(nil) creates a copy of the value bytes for safe use outside transaction
			blockData, _ := item.ValueCopy(nil)

			// Deserialize the byte data back into a Block struct
			// The asterisk (*) dereferences the pointer returned by Deserialize
			block = *Deserialize(blockData)
		}
		return nil // Return nil to indicate successful transaction
	})

	if err != nil {
		return block, err
	}

	return block, nil
}

// GetBlockHashes returns a list of all block hashes in the blockchain
// This function iterates through the entire chain from newest to oldest block
// Useful for:
// 1. Blockchain synchronization between nodes
// 2. Creating block inventories for peer-to-peer sharing
// 3. Verification and analysis of chain structure
func (chain *BlockChain) GetBlockHashes() [][]byte {
	var blocks [][]byte // Slice to collect all block hashes

	// Create an iterator that traverses the blockchain in reverse chronological order
	// (from newest/current tip back to genesis block)
	iter := chain.Iterator()

	// Iterate through all blocks in the chain
	for {
		// Get the next block in the iteration (starts with the current tip)
		block := iter.Next()

		// Add the block's hash to our collection
		// The hash serves as a unique identifier for each block
		blocks = append(blocks, block.Hash)

		// Check if we've reached the genesis block
		//  has an empty previous hash (length 0)
		// This is our termination condition
		if len(block.PrevHash) == 0 {
			break // Stop iteration - reached beginning of a chain
		}
	}

	// Return the collected block hashes
	// Note: The hashes are in REVERSE order (newest first, oldest last)
	return blocks
}

// MineBlock creates a new block containing validated transactions and adds it to the blockchain
// This is the core mining function that:
// 1. Validates all input transactions
// 2. Retrieves current blockchain state (last block info)
// 3. Creates a new block with the transactions
// 4. Updates the blockchain database with the new block
func (chain *BlockChain) MineBlock(transactions []*Transaction) *Block {
	var lastHash []byte // Hash of the most recent block in the chain
	var lastHeight int  // Height/number of the most recent block

	// Validate every transaction before including it in the block
	// This prevents invalid transactions from being permanently recorded on the blockchain
	for _, tx := range transactions {
		// Check if each transaction is cryptographically valid and follows blockchain rules
		if chain.VerifyTransaction(tx) != true {
			log.Panic("Invalid Transaction") // Stop everything if any transaction is invalid
		}
	}

	// Read the current blockchain state from the database
	// Using a read-only transaction to safely retrieve the last block information
	err := chain.Database.View(func(txn *badger.Txn) error {
		// Step 1: Get the "last hash" pointer (key "lh" stores hash of the most recent block)
		item, err := txn.Get([]byte("lh"))
		Handle(err) // Exit if you can't retrieve the last hash pointer

		// Retrieve the actual hash value from the database item
		// ValueCopy(nil) returns a copy of the value as a byte slice for BadgerDB v4
		lastHash, err = item.ValueCopy(nil)
		Handle(err) // Exit if you can't read the hash value

		// Step 2: Get the actual last block using its hash as the key
		item, err = txn.Get(lastHash)
		Handle(err) // Exit if the last block doesn't exist (shouldn't happen in a valid chain)

		// Retrieve the serialized block data
		lastBlockData, _ := item.ValueCopy(nil) // Get block bytes, ignore the error (already handled above)

		// Step 3: Convert serialized bytes back into Block struct
		lastBlock := Deserialize(lastBlockData)

		// Step 4: Extract the block height to calculate the next block height
		lastHeight = lastBlock.Height

		return err // Return any accumulated error
	})
	Handle(err) // Exit if any database operation failed

	// Create the new block with:
	// - The validated transactions
	// - Reference to the previous block's hash (lastHash)
	// - Next sequential height (lastHeight + 1)
	newBlock := CreateBlock(transactions, lastHash, lastHeight+1)

	// Write the new block to the database
	// Using a read-write transaction to update the blockchain state
	err = chain.Database.Update(func(txn *badger.Txn) error {
		// Step 1: Store the new block using its hash as the key
		// This allows a quick lookup of any block by its hash
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		Handle(err) // Exit if you can't store a block

		// Step 2: Update the "last hash" pointer to point to this new block
		// This is how the chain maintains its current tip/head
		err = txn.Set([]byte("lh"), newBlock.Hash)
		Handle(err) // Exit if can't update pointer

		// Step 3: Update in-memory reference for faster later access
		// This avoids needing to read from a database for the next mining operation
		chain.LastHash = newBlock.Hash

		return err // Return any accumulated error
	})
	Handle(err) // Exit if any database update failed

	// Return the newly created and stored block
	return newBlock
}

// AddBlock adds an existing block to the blockchain
// This function is used when receiving blocks from other nodes in the network
// Unlike MineBlock, it doesn't create a block, just validates and stores it
func (chain *BlockChain) AddBlock(block *Block) {
	// Write transaction to potentially add the block
	err := chain.Database.Update(func(txn *badger.Txn) error {
		// Step 1: Check if a block already exists in the database
		// This prevents duplicate blocks and wasted storage
		if _, err := txn.Get(block.Hash); err == nil {
			// Block already exists, no need to add it again
			return nil // Early exit - block is already in chain
		}

		// Step 2: Serialize and store the new block
		// Convert block struct to byte format for storage
		blockData := block.Serialize()
		err := txn.Set(block.Hash, blockData)
		Handle(err) // Exit if you can't store a block

		// Step 3: Check if this block should become the new chain tip
		// We only update the tip if this block builds on the current longest chain
		item, err := txn.Get([]byte("lh"))
		Handle(err) // Exit if can't get current tip pointer

		// Get the current tip's hash
		lastHash, err := item.ValueCopy(nil)
		Handle(err) // Exit if you can't read the current tip hash

		// Step 4: Get the current tip block to compare heights
		item, err = txn.Get(lastHash)
		Handle(err) // Exit if the current tip block doesn't exist

		lastBlockData, _ := item.ValueCopy(nil) // Get current tip block data
		lastBlock := Deserialize(lastBlockData) // Convert to Block struct

		// Step 5: Update chain tip if the new block has greater height
		// This implements the "longest chain" rule of blockchain consensus
		if lastBlock.Height < block.Height {
			// The new block is on a longer chain, update the tip
			err = txn.Set([]byte("lh"), block.Hash)
			Handle(err) // Exit if can't update tip pointer

			// Update in-memory reference for consistency
			chain.LastHash = block.Hash
		}

		// Return success - block was either added or already existed
		return nil
	})
	Handle(err) // Exit if any database operation failed
}

// FindUTXO scans the entire blockchain to build a complete map of all unspent transaction outputs
// This is used to initialize or rebuild the UTXO set index
// Returns: map[TransactionID] -> TxOutputs (collection of unspent outputs for that transaction)
func (chain *BlockChain) FindUTXO() map[string]TxOutputs {
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
func (bc *BlockChain) FindTransaction(ID []byte) (Transaction, error) {
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
func (bc *BlockChain) SignTransaction(tx *Transaction, privateKey ecdsa.PrivateKey) {
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
func (bc *BlockChain) VerifyTransaction(tx *Transaction) bool {
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

func retry(dir string, originalOpts badger.Options) (*badger.DB, error) {
	lockPath := filepath.Join(dir, "LOCK")
	if err := os.Remove(lockPath); err != nil {
		return nil, fmt.Errorf("failed to remove lock file: %w", err)
	}
	retryOpts := originalOpts
	db, err := badger.Open(retryOpts)
	return db, err
}

func openDB(dir string, opts badger.Options) (*badger.DB, error) {
	if db, err := badger.Open(opts); err != nil {
		if strings.Contains(err.Error(), "LOCK") {
			if db, err = retry(dir, opts); err == nil {
				log.Println("database unlocked")
				return db, nil
			}
			log.Println("could not unlock database: ", err)
		}
		return nil, err
	} else {
		return db, nil
	}
}
