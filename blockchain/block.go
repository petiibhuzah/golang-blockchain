package blockchain

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 26/11/2025
 * Time: 11:20
 */

type Block struct {
	Timestamp    int64
	Hash         []byte         // Hash representing this block
	Transactions []*Transaction // The Data that this block stored. Transaction/Record/Document
	PrevHash     []byte         // The Hash of the previous block in a BlockChain
	Nonce        int            // The Nonce for validation of proof or work in a mining process
	Height       int
}

// HashTransactions Special function for hashing the transactions in a block for PoW validation
func (b *Block) HashTransactions() []byte {
	var txHashes [][]byte

	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.Serialize())
	}

	tree := NewMerkleTree(txHashes)
	return tree.RootNode.Data
}

// CreateBlock Special function for creating block
func CreateBlock(txs []*Transaction, prevHash []byte, height int) *Block {
	block := &Block{Timestamp: time.Now().Unix(), Hash: []byte{}, Transactions: txs, PrevHash: prevHash, Nonce: 0, Height: height} // Using block constructor
	pow := NewProof(block)
	nonce, hash := pow.Run()

	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}

// Genesis Special function for creating the genesis block
func Genesis(coinbase *Transaction) *Block {
	return CreateBlock([]*Transaction{coinbase}, []byte{}, 0)
}

// Serialize Special function for serializing the data before storing to the key value database badgerDB
func (b *Block) Serialize() []byte {
	var res bytes.Buffer
	encoder := gob.NewEncoder(&res)
	err := encoder.Encode(b)
	if err != nil {
		Handle(err)
	}
	return res.Bytes()
}

// Deserialize Special function for decoding the data retrieved from the key value database badgerDB
func Deserialize(data []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(data))

	err := decoder.Decode(&block)
	if err != nil {
		Handle(err)
	}
	return &block
}

func Handle(err error) {
	if err != nil {
		log.Panic(err)
	}
}
