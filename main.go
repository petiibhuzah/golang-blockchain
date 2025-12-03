package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/dgraph-io/badger/v4"
	"github.com/golang-blockchain/blockchain"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 26/11/2025
 * Time: 10:29
 */

type CommandLine struct {
	blockchain *blockchain.Blockchain
}

func (cli *CommandLine) printUsage() {
	fmt.Println("Usage:")
	fmt.Println(" add - block BLOCK_DATA - add a block to the chain")
	fmt.Println(" print - Prints the blocks in the chain")
}

// Special function for validating the arguments passed in CLI
func (cli *CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit() // Exit the application by shutting down the go routine with perfect garbage collection preventing the databased for collapsing
	}
}

func (cli *CommandLine) printChain() {
	iter := cli.blockchain.Iterator()

	for {
		block := iter.Next()

		fmt.Printf("Previous Hash: %x\n", block.PrevHash)
		fmt.Printf("Block Data: %s\n", block.Data)
		fmt.Printf("Hash: %x\n\n", block.Hash)

		pow := blockchain.NewProof(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		if len(block.PrevHash) == 0 {
			break // Means we have reached the genesis block
		}
	}
}

func (cli *CommandLine) Run() {
	cli.validateArgs()

	// Parse the command line arguments
	addBlockCMD := flag.NewFlagSet("add", flag.ExitOnError)
	printChainCMD := flag.NewFlagSet("print", flag.ExitOnError)
	addBlockData := addBlockCMD.String("block", "", "Block Data")

	switch os.Args[1] {
	case "add":
		err := addBlockCMD.Parse(os.Args[2:])
		blockchain.Handle(err)
	case "print":
		err := printChainCMD.Parse(os.Args[2:])
		blockchain.Handle(err)
	default:
		cli.printUsage()
		runtime.Goexit()
	}

	if addBlockCMD.Parsed() {
		if *addBlockData == "" {
			addBlockCMD.Usage()
			runtime.Goexit()
		}
		cli.AddBlock(*addBlockData)
	}

	if printChainCMD.Parsed() {
		cli.printChain()
	}
}

// AddBlock Special function for adding a block to the blockchain
func (cli *CommandLine) AddBlock(data string) {
	cli.blockchain.AddBlock(data)
	fmt.Println("Block successfully added!")
}

func main() {
	defer os.Exit(0)
	chain := blockchain.InitBlockChain()

	defer func(Database *badger.DB) {
		err := Database.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(chain.Database)

	cli := CommandLine{chain}
	cli.Run()
}
