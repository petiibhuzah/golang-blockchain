package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 26/11/2025
 * Time: 10:29
 */

type Blockchain struct {
	blocks []*Block
}
type Block struct {
	Hash     []byte // Hash representing this block
	Data     []byte // The Data that this block stored. Transaction/Record/Document
	PrevHash []byte // The Hash of the previous block in a Blockchain
}

// DeriveHash Special method to get the block hash
func (b *Block) DeriveHash() {
	info := bytes.Join([][]byte{b.Data, b.PrevHash}, []byte{}) // 2 Dimension of a combination of b.Data and b.PrevHash, then we combine with an empty slice of byte
	hash := sha256.Sum256(info)                                // Creating the actual hash [Hash is a simple way to calculate the block has there is another real way we will implement later]
	b.Hash = hash[:]                                           // Pushing the hash we have created to our block
}

// CreateBlock Special function for creating block
func CreateBlock(data string, prevHash []byte) *Block {
	block := &Block{[]byte{}, []byte(data), prevHash} // Using block constructor
	block.DeriveHash()
	return block
}

// AddBlock Special function for adding the block to the blockchain
func (chain *Blockchain) AddBlock(data string) {
	prevBlock := chain.blocks[len(chain.blocks)-1] // Getting the previous block
	newBlock := CreateBlock(data, prevBlock.Hash)  // Creating the new block and using the previous block hash
	chain.blocks = append(chain.blocks, newBlock)  // Appending the new block in a blockchain
}

// Genesis Special function for creating the genesis block
func Genesis() *Block {
	return CreateBlock("Genesis Block", []byte{})
}

func InitBlockChain() *Blockchain {
	return &Blockchain{[]*Block{Genesis()}} // We return the reference of the blockchain with a call to a function generating genesis block
}
func main() {
	chain := InitBlockChain()

	chain.AddBlock("First Block After Genesis")
	chain.AddBlock("second Block After First Block")
	chain.AddBlock("Third Block After Second Block")
	for _, block := range chain.blocks {
		fmt.Printf("Previous Hash: %x\n", block.PrevHash)
		fmt.Printf("Block Data: %s\n", block.Data)
		fmt.Printf("Block Hash: %x\n\n", block.Hash)
	}
}
