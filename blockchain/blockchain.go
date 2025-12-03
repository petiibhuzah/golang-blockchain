package blockchain

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 29/11/2025
 * Time: 14:21
 */

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

const (
	dbPath = ".tmp/blocks"
)

type Blockchain struct {
	LastHash []byte     // The hash of the last block in the blockchain
	Database *badger.DB // The database for storing the blockchain
}

// Iterator Struct for iterating through the blockchain in badger database
type Iterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

func InitBlockChain() *Blockchain {
	var lastHash []byte

	opts := badger.DefaultOptions(dbPath)
	opts.Dir = dbPath      // Key and metadata will be stored in this directory
	opts.ValueDir = dbPath // Value will be stored in this directory

	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		// Check if there is an existing blockchain | Genesis block
		if _, err := txn.Get([]byte("lh")); errors.Is(err, badger.ErrKeyNotFound) {
			fmt.Println("No existing blockchain found, creating new blockchain...")
			genesis := Genesis()
			fmt.Println("Genesis block created")
			err = txn.Set(genesis.Hash, genesis.Serialize())
			Handle(err)
			err = txn.Set([]byte("lh"), genesis.Hash)
			lastHash = genesis.Hash
			return err
		} else {
			// In case there is an existing blockchain
			item, err := txn.Get([]byte("lh"))
			Handle(err)
			err = item.Value(func(val []byte) error {
				lastHash = val
				return nil
			})
			return err
		}
	})
	Handle(err)

	blockchain := Blockchain{lastHash, db}
	return &blockchain
}

// AddBlock Special function for adding the block to the blockchain
func (chain *Blockchain) AddBlock(data string) {
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

	newBlock := CreateBlock(data, lastHash)

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

	iter.CurrentHash = block.PrevHash // Since is going backward until genesis block
	return block
}
