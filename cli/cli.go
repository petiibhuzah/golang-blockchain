package cli

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/dgraph-io/badger/v4"
	"github.com/golang-blockchain/blockchain"
	"github.com/golang-blockchain/network"
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
	fmt.Println(" createblockchain -address ADDRESS - create a blockchain")
	fmt.Println(" printblockchain - Print the blocks in a the chain")
	fmt.Println(" send -from FROM -to TO -amount AMOUNT -mine - Send coins from one address to another. Then -mine flag is set, mine off of this node")
	fmt.Println(" createwallet - Create a new wallet")
	fmt.Println(" listaddresses - Lists the addresses in our wallet file")
	fmt.Println(" reindexutxo - Rebuilds the UTXO set")
	fmt.Println(" startnode -miner ADDRESS - Start a node specified in NODE_ID env. var. -miner enables mining")
}

// Special function for validating the arguments passed in CLI
func (cli *CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit() // Exit the application by shutting down the go routine with perfect garbage collection preventing the databased for collapsing
	}
}

func (cli *CommandLine) StartNode(nodeID, minerAddress string) {
	fmt.Printf("Starting Node %s\n", nodeID)

	if len(minerAddress) > 0 {
		if wallet.ValidateAddress(minerAddress) {
			fmt.Println("Mining is on. Address to receive rewards: ", minerAddress)
		} else {
			log.Panic("Wrong miner address!", minerAddress)
		}
	}

	network.StartServer(nodeID, minerAddress)
}

func (cli *CommandLine) printChain(nodeID string) {
	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close()

	iter := chain.Iterator()

	for {
		block := iter.Next()

		fmt.Printf("Prev. hash: %x\n", block.PrevHash)
		fmt.Printf("Hash: %v\n", block.Hash)
		pow := blockchain.NewProof(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		for _, tx := range block.Transactions {
			fmt.Printf("Transaction: %s\n", tx)
		}
		fmt.Println()

		if len(block.PrevHash) == 0 {
			break // Means we have reached the genesis block
		}
	}
}

func (cli *CommandLine) createBlockChain(address, nodeID string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Invalid address!")
	}

	chain := blockchain.InitBlockChain(address, nodeID)
	defer func(Database *badger.DB) {
		err := Database.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(chain.Database)

	UTXOSet := blockchain.UTXOSet{Blockchain: chain}
	UTXOSet.Reindex()

	fmt.Println("Finished creating blockchain!")
}

func (cli *CommandLine) getBalance(address, nodeID string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Invalid address!")
	}

	chain := blockchain.ContinueBlockChain(nodeID)
	UTXOSet := blockchain.UTXOSet{Blockchain: chain}
	defer func(Database *badger.DB) {
		err := Database.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(chain.Database)

	balance := 0

	pubKeyHash := wallet.Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4] // Remove version [1 first byte] and checksum [4 last bytes]
	UTXOs := UTXOSet.FindUnspentTransactions(pubKeyHash)

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("Balance of %s: TZS %d\n", address, balance)
}

func (cli *CommandLine) send(from, to string, amount int, nodeID string, mineNow bool) {
	if !wallet.ValidateAddress(from) {
		log.Panic("Invalid from address!")
	}

	if !wallet.ValidateAddress(to) {
		log.Panic("Invalid to address!")
	}

	chain := blockchain.ContinueBlockChain(nodeID)
	UTXOSet := blockchain.UTXOSet{Blockchain: chain}
	defer func(Database *badger.DB) {
		err := Database.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(chain.Database)

	wallets, err := wallet.CreateWallets(nodeID)
	if err != nil {
		log.Panic(err)
	}
	w := wallets.GetWallet(from)

	tx := blockchain.NewTransaction(&w, to, amount, &UTXOSet)
	if mineNow {
		cbTx := blockchain.CoinbaseTx(from, "")
		txs := []*blockchain.Transaction{cbTx, tx}
		block := chain.MineBlock(txs)
		UTXOSet.Update(block)
	} else {
		network.SendTx(network.KnownNodes[0], tx)
		fmt.Println("Send tx")
	}

	fmt.Println("Success!")
}

func (cli *CommandLine) reindexUTXO(nodeID string) {
	chain := blockchain.ContinueBlockChain(nodeID)
	defer func(Database *badger.DB) {
		err := Database.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(chain.Database)

	UTXOSet := blockchain.UTXOSet{Blockchain: chain}
	UTXOSet.Reindex()

	count := UTXOSet.CountTransactions()
	fmt.Printf("Done! There are %d transactions in the UTXO set.\n", count)
}

func (cli *CommandLine) listAddresses(nodeID string) {
	wallets, _ := wallet.CreateWallets(nodeID)
	addresses := wallets.GetAllAddresses()

	for _, address := range addresses {
		fmt.Println(address)
	}
}

func (cli *CommandLine) createWallet(nodeID string) {
	wallets, _ := wallet.CreateWallets(nodeID)
	address := wallets.AddWallet()
	wallets.SaveFile(nodeID)
	fmt.Printf("New wallet created with address: %s\n", address)
}

func (cli *CommandLine) Run() {
	cli.validateArgs()

	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		fmt.Printf("NODE_ID env is not set!")
		runtime.Goexit()
	}

	// Parse the command line arguments
	getBalanceCMD := flag.NewFlagSet("getbalance", flag.ExitOnError)
	createBlockChainCMD := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	sendCMD := flag.NewFlagSet("send", flag.ExitOnError)
	printChainCMD := flag.NewFlagSet("printchain", flag.ExitOnError)
	createWalletCMD := flag.NewFlagSet("createwallet", flag.ExitOnError)
	listAddressesCMD := flag.NewFlagSet("listaddresses", flag.ExitOnError)
	reindexUTXOCMD := flag.NewFlagSet("reindexutxo", flag.ExitOnError)
	startNodeCMD := flag.NewFlagSet("startnode", flag.ExitOnError)

	getBalanceAddress := getBalanceCMD.String("address", "", "Wallet address to get the balance of")
	createBlockChainAddress := createBlockChainCMD.String("address", "", "Wallet address to create the blockchain for")
	sendFrom := sendCMD.String("from", "", "Source wallet address")
	sendTo := sendCMD.String("to", "", "Destination wallet address")
	sendAmount := sendCMD.Int("amount", 0, "Amount to send")
	sendMine := sendCMD.Bool("mine", false, "Mine immediately no the same node")
	startNodeMiner := startNodeCMD.String("miner", "", "Enable mining mode and send reward to ADDRESS")

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
	case "reindexutxo":
		err := reindexUTXOCMD.Parse(os.Args[2:])
		blockchain.Handle(err)
	case "startnode":
		err := startNodeCMD.Parse(os.Args[2:])
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
		cli.getBalance(*getBalanceAddress, nodeID)
	}

	if createBlockChainCMD.Parsed() {
		if *createBlockChainAddress == "" {
			createBlockChainCMD.Usage()
			runtime.Goexit()
		}
		cli.createBlockChain(*createBlockChainAddress, nodeID)
	}

	if printChainCMD.Parsed() {
		cli.printChain(nodeID)
	}

	if createWalletCMD.Parsed() {
		cli.createWallet(nodeID)
	}

	if listAddressesCMD.Parsed() {
		cli.listAddresses(nodeID)
	}

	if reindexUTXOCMD.Parsed() {
		cli.reindexUTXO(nodeID)
	}

	if sendCMD.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCMD.Usage()
			runtime.Goexit()
		}
		cli.send(*sendFrom, *sendTo, *sendAmount, nodeID, *sendMine)
	}

	if startNodeCMD.Parsed() {
		nID := os.Getenv("NODE_ID")
		if nID == "" {
			startNodeCMD.Usage()
			runtime.Goexit()
		}
		cli.StartNode(nID, *startNodeMiner)
	}
}
