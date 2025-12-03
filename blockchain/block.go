package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 26/11/2025
 * Time: 11:20
 */

type Block struct {
	Hash     []byte // Hash representing this block
	Data     []byte // The Data that this block stored. Transaction/Record/Document
	PrevHash []byte // The Hash of the previous block in a Blockchain
	Nonce    int    // The Nonce for validation of proof or work in a mining process
}

// DeriveHash Special method to get the block hash
func (b *Block) DeriveHash() {
	info := bytes.Join([][]byte{b.Data, b.PrevHash}, []byte{}) // 2 Dimension of a combination of b.Data and b.PrevHash, then we combine with an empty slice of byte
	hash := sha256.Sum256(info)                                // Creating the actual hash [Hash is a simple way to calculate the block has there is another real way we will implement later]
	b.Hash = hash[:]                                           // Pushing the hash we have created to our block
}

// CreateBlock Special function for creating block
func CreateBlock(data string, prevHash []byte) *Block {
	block := &Block{[]byte{}, []byte(data), prevHash, 0} // Using block constructor
	pow := NewProof(block)
	nonce, hash := pow.Run()
	block.Hash = hash[:]
	block.Nonce = nonce
	return block
}

// Genesis Special function for creating the genesis block
func Genesis() *Block {
	return CreateBlock("Genesis Block", []byte{})
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
