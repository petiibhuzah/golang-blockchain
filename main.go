package main

import (
	"fmt"
	"github.com/golang-blockchain/blockchain"
	"strconv"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 26/11/2025
 * Time: 10:29
 */

func main() {
	chain := blockchain.InitBlockChain()

	chain.AddBlock("First Block After Genesis")
	chain.AddBlock("second Block After Genesis")
	chain.AddBlock("Third Block After Genesis")
	
	for _, block := range chain.Blocks {
		fmt.Printf("Previous Hash: %x\n", block.PrevHash)
		fmt.Printf("Block Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n\n", block.Hash)

		pow := blockchain.NewProof(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()
	}
}
