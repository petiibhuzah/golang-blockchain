package blockchain

import (
	"bytes"
	"crypto/sha256"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 26/11/2025
 * Time: 11:20
 */

type Blockchain struct {
	Blocks []*Block
}
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

// AddBlock Special function for adding the block to the blockchain
func (chain *Blockchain) AddBlock(data string) {
	prevBlock := chain.Blocks[len(chain.Blocks)-1] // Getting the previous block
	newBlock := CreateBlock(data, prevBlock.Hash)  // Creating the new block and using the previous block hash
	chain.Blocks = append(chain.Blocks, newBlock)  // Appending the new block in a blockchain
}

// Genesis Special function for creating the genesis block
func Genesis() *Block {
	return CreateBlock("Genesis Block", []byte{})
}

func InitBlockChain() *Blockchain {
	return &Blockchain{[]*Block{Genesis()}} // We return the reference of the blockchain with a call to a function generating genesis block
}
