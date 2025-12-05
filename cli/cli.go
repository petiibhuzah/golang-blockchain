package cli

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/dgraph-io/badger/v4"
	"github.com/golang-blockchain/blockchain"
	"github.com/golang-blockchain/wallet"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 04/12/2025
 * Time: 16:42
 */

type CommandLine struct{}

func (cli *CommandLine) printUsage() {
	fmt.Println("Usage:")
	fmt.Println(" getbalance -address ADDRESS - get the balance of an address")
	fmt.Println(" createblockchain -address ADDRESS - create a blockchain") // We create the blockchain and the blockchain mines the genesis of that blockchain [Key point]
	fmt.Println(" printblockchain - Print the blocks in a the chain")
	fmt.Println(" send -from FROM -to TO -amount AMOUNT - Send coins from one address to another")
	fmt.Println(" createwallet - Create a new wallet")
	fmt.Println(" listaddresses - Lists the addresses in our wallet file")
}

// Special function for validating the arguments passed in CLI
func (cli *CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit() // Exit the application by shutting down the go routine with perfect garbage collection preventing the databased for collapsing
	}
}

func (cli *CommandLine) printChain() {
	chain := blockchain.ContinueBlockChain("") // Since the Genesis Block already exists, we don't need to create a new blockchain'
	defer chain.Database.Close()

	iter := chain.Iterator()

	for {
		block := iter.Next()

		fmt.Printf("Prev. hash: %x\n", block.PrevHash)
		fmt.Printf("Hash: %v\n", block.Hash)
		pow := blockchain.NewProof(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		if len(block.PrevHash) == 0 {
			break // Means we have reached the genesis block
		}
	}
}

func (cli *CommandLine) createBlockChain(address string) {
	chain := blockchain.InitBlockChain(address)
	defer func(Database *badger.DB) {
		err := Database.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(chain.Database)
	fmt.Println("Finished creating blockchain!")
}

func (cli *CommandLine) getBalance(address string) {
	chain := blockchain.ContinueBlockChain(address)
	defer func(Database *badger.DB) {
		err := Database.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(chain.Database)

	balance := 0
	UTXOs := chain.FindUTXO(address)

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("Balance of %s: TZS %d\n", address, balance)
}

func (cli *CommandLine) send(from, to string, amount int) {
	chain := blockchain.ContinueBlockChain(from)
	defer func(Database *badger.DB) {
		err := Database.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(chain.Database)

	tx := blockchain.NewTransaction(from, to, amount, chain)
	chain.AddBlock([]*blockchain.Transaction{tx})
	fmt.Println("Transaction successful!")
}

func (cli *CommandLine) listAddresses() {
	wallets, _ := wallet.CreateWallets()
	addresses := wallets.GetAllAddresses()

	for _, address := range addresses {
		fmt.Println(address)
	}
}

func (cli *CommandLine) createWallet() {
	wallets, _ := wallet.CreateWallets()
	address := wallets.AddWallet()
	wallets.SaveFile()
	fmt.Printf("New wallet created with address: %s\n", address)
}

func (cli *CommandLine) Run() {
	cli.validateArgs()

	// Parse the command line arguments
	getBalanceCMD := flag.NewFlagSet("getbalance", flag.ExitOnError)
	createBlockChainCMD := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	sendCMD := flag.NewFlagSet("send", flag.ExitOnError)
	printChainCMD := flag.NewFlagSet("printchain", flag.ExitOnError)
	createWalletCMD := flag.NewFlagSet("createwallet", flag.ExitOnError)
	listAddressesCMD := flag.NewFlagSet("listaddresses", flag.ExitOnError)

	getBalanceAddress := getBalanceCMD.String("address", "", "Wallet address to get the balance of")
	createBlockChainAddress := createBlockChainCMD.String("address", "", "Wallet address to create the blockchain for")
	sendFrom := sendCMD.String("from", "", "Source wallet address")
	sendTo := sendCMD.String("to", "", "Destination wallet address")
	sendAmount := sendCMD.Int("amount", 0, "Amount to send")

	switch os.Args[1] {
	case "getbalance":
		err := getBalanceCMD.Parse(os.Args[2:])
		blockchain.Handle(err)
	case "createblockchain":
		err := createBlockChainCMD.Parse(os.Args[2:])
		blockchain.Handle(err)
	case "send":
		err := sendCMD.Parse(os.Args[2:])
		blockchain.Handle(err)
	case "printchain":
		err := printChainCMD.Parse(os.Args[2:])
		blockchain.Handle(err)
	case "createwallet":
		err := createWalletCMD.Parse(os.Args[2:])
		blockchain.Handle(err)
	case "listaddresses":
		err := listAddressesCMD.Parse(os.Args[2:])
		blockchain.Handle(err)
	default:
		cli.printUsage()
		runtime.Goexit()
	}

	if getBalanceCMD.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCMD.Usage()
			runtime.Goexit()
		}
		cli.getBalance(*getBalanceAddress)
	}

	if createBlockChainCMD.Parsed() {
		if *createBlockChainAddress == "" {
			createBlockChainCMD.Usage()
			runtime.Goexit()
		}
		cli.createBlockChain(*createBlockChainAddress)
	}

	if printChainCMD.Parsed() {
		cli.printChain()
	}

	if createWalletCMD.Parsed() {
		cli.createWallet()
	}

	if listAddressesCMD.Parsed() {
		cli.listAddresses()
	}

	if sendCMD.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCMD.Usage()
			runtime.Goexit()
		}
		cli.send(*sendFrom, *sendTo, *sendAmount)
	}
}
