package blockchain

import "github.com/dgraph-io/badger/v4"

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 11/12/2025
 * Time: 14:35
 */

// Iterator Struct for iterating through the blockchain in a badger database
type Iterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

// Iterator Special function for creating an iterator for iterating through the blockchain
func (chain *BlockChain) Iterator() *Iterator {
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
