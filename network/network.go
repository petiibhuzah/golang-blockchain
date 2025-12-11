package network

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"

	"github.com/golang-blockchain/blockchain"
	"github.com/vrecan/death/v3"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 11/12/2025
 * Time: 11:12
 */

// ============================================================================
// NETWORK CONSTANTS & GLOBAL VARIABLES
// ============================================================================
/*
   How the Network Works:
	1. Node Startup:
	   - Node starts on localhost:3000
	   - Loads blockchain from database
	   - Starts listening for connections

	2. Node Discovery:
	   - Connects to bootstrap node (localhost:3000)
	   - Sends "version" message with blockchain height
	   - Receives "addr" messages with other node addresses

	3. Blockchain Sync:
	   - If behind: sends "getblocks" to request block hashes
	   - Receives "inv" with block hashes
	   - Requests missing blocks with "getdata"
	   - Receives blocks with "block" messages
	   - Adds blocks to a local chain

	4. Transaction Propagation:
	   - User creates transaction
	   - Node sends "tx" message to peers
	   - Peers validate and add to the memory pool
	   - Miner nodes bundle transactions into blocks
	   - Miner broadcasts a new block with "inv"

	5. Mining:
	   - Miner collects transactions from the memory pool
	   - Creates coinbase transaction (mining reward)
	   - Solves Proof-of-Work puzzle
	   - Broadcasts new block to network
	   - Other nodes validate and add to their chains


┌─────────────────────────────────────────────────────────────┐
│                    BLOCKCHAIN NETWORK                       │
├─────────────┬──────────────┬──────────────┬─────────────────┤
│  Full Nodes │ Mining Nodes │  Light Nodes │ Archive Nodes   │
│   (Heavy)   │   (Miners)   │  (SPV/Wallet)│  (Historical)   │
├─────────────┼──────────────┼──────────────┼─────────────────┤
│ Validate    │ Create Blocks│ Verify Tx    │ Store Full      │
│ All Rules   │ Solve PoW/PoS│ Using Merkle │ History         │
│ Store Full  │ Earn Rewards │ Proofs       │ Pruned Data     │
│ Blockchain  │              │ Mobile Use   │ Research        │
└─────────────┴──────────────┴──────────────┴─────────────────┘

NOTE: SVP - Simplified Payment Verification
*/

// Network protocol constants define how nodes communicate
const (
	protocol      = "tcp" // Transport protocol (TCP for reliability)
	version       = 1     // Network protocol version (for backward compatibility)
	commandLength = 12    // Fixed the length for command names in messages
)

// Global network state variables
var (
	nodeAddress     string                                    // This node's address (e.g., "localhost:3000")
	mineAddress     string                                    // Miner's reward address (if this node mines)
	KnownNodes      = []string{"localhost:3000"}              // Bootstrap node list - starts with the central seed node
	blocksInTransit = [][]byte{}                              // Blocks we're currently downloading
	memoryPool      = make(map[string]blockchain.Transaction) // Unconfirmed transactions waiting for mining
)

// ============================================================================
// NETWORK MESSAGE STRUCTURES (P2P Protocol Messages)
// ============================================================================

// Addr message broadcasts known node addresses to peers (node discovery)
type Addr struct {
	AddrList []string // List of known node addresses in the network
}

// Block message sends a complete serialized block to peers
type Block struct {
	AddrFrom string // Sender's address
	Block    []byte // Serialized block data
}

// GetBlocks message requests block hashes from a peer (inventory discovery)
type GetBlocks struct {
	AddrFrom string // Requestor's address
}

// GetData message requests specific data (block or transaction) from a peer
type GetData struct {
	AddrFrom string // Requestor's address
	Type     string // "block" or "tx" - type of data requested
	ID       []byte // Hash of the requested block or transaction
}

// Inv (Inventory) message advertises available data to peers
type Inv struct {
	AddrFrom string   // Sender's address
	Type     string   // "block" or "tx" - type of inventory items
	Items    [][]byte // List of block/transaction hashes available
}

// Tx message broadcasts a transaction to the network
type Tx struct {
	AddrFrom    string // Sender's address
	Transaction []byte // Serialized transaction data
}

// Version message exchanges version information when nodes connect (handshake)
type Version struct {
	Version    int    // Protocol version for compatibility checking
	BestHeight int    // Height of sender's blockchain (for syncing)
	AddrFrom   string // Sender's address
}

// ============================================================================
// NETWORK PROTOCOL UTILITIES
// ============================================================================

// CmdToBytes converts a command string to fixed-length byte array
// Ensures a consistent command format across the network
func CmdToBytes(cmd string) []byte {
	var b [commandLength]byte // Fixed the 12-byte array

	// Copy command characters into an array
	for i, c := range cmd {
		b[i] = byte(c)
	}
	return b[:] // Return as a slice
}

// BytesToCmd converts a byte array back to command string
// Used to parse incoming network messages
func BytesToCmd(bytes []byte) string {
	var cmd []byte

	// Skip null bytes (padding in fixed-length command)
	for _, b := range bytes {
		if b != 0x0 {
			cmd = append(cmd, b)
		}
	}
	return fmt.Sprintf("%s", cmd)
}

// ExtractCmd extracts the command from a network message
// First 12 bytes of every message contain the command
func ExtractCmd(request []byte) []byte {
	return request[0:commandLength]
}

// ============================================================================
// NETWORK MESSAGE SENDING FUNCTIONS
// ============================================================================

// RequestBlocks asks all known nodes for their block inventories
// Used during initial sync to discover missing blocks
func RequestBlocks() {
	for _, node := range KnownNodes {
		SendGetBlocks(node)
	}
}

// SendAddr broadcasts our known node list to a peer
// Helps with peer discovery and network connectivity
func SendAddr(address string) {
	nodes := Addr{KnownNodes}
	nodes.AddrList = append(nodes.AddrList, nodeAddress) // Include ourselves
	payload := GobEncode(nodes)
	request := append(CmdToBytes("addr"), payload...)

	SendData(address, request)
}

// SendBlock sends a serialized block to a specific node
func SendBlock(addr string, b *blockchain.Block) {
	data := Block{AddrFrom: nodeAddress, Block: b.Serialize()}
	payload := GobEncode(data)
	request := append(CmdToBytes("block"), payload...)

	SendData(addr, request)
}

// SendData is the low-level function that transmits data over TCP
// Handles connection errors and updates a known nodes list
func SendData(addr string, data []byte) {
	conn, err := net.Dial(protocol, addr)

	if err != nil {
		fmt.Printf("%s is not available\n", addr)

		// Remove dead node from known nodes
		var updatedNodes []string
		for _, node := range KnownNodes {
			if node != addr {
				updatedNodes = append(updatedNodes, node)
			}
		}
		KnownNodes = updatedNodes
		return
	}

	defer conn.Close()
	_, err = io.Copy(conn, bytes.NewReader(data))
	if err != nil {
		log.Panic(err)
	}
}

// SendGetBlocks requests block hashes from a node
// First step in blockchain synchronization
func SendGetBlocks(address string) {
	payload := GobEncode(GetBlocks{AddrFrom: nodeAddress})
	request := append(CmdToBytes("getblocks"), payload...)

	SendData(address, request)
}

// SendGetData requests specific data (block or transaction) by hash
func SendGetData(address, kind string, id []byte) {
	payload := GobEncode(GetData{AddrFrom: nodeAddress, Type: kind, ID: id})
	request := append(CmdToBytes("getdata"), payload...)

	SendData(address, request)
}

// SendInv advertises available inventory (blocks or transactions)
// Used to inform peers what data we have
func SendInv(address, kind string, items [][]byte) {
	inventory := Inv{AddrFrom: nodeAddress, Type: kind, Items: items}
	payload := GobEncode(inventory)
	request := append(CmdToBytes("inv"), payload...)

	SendData(address, request)
}

// SendTx broadcasts a transaction to the network
func SendTx(address string, tx *blockchain.Transaction) {
	data := Tx{AddrFrom: nodeAddress, Transaction: tx.Serialize()}
	payload := GobEncode(data)
	request := append(CmdToBytes("tx"), payload...)

	SendData(address, request)
}

// SendVersion exchanges version information during handshake
// Critical for determining which node has the longer blockchain
func SendVersion(address string, chain *blockchain.Blockchain) {
	bestHeight := chain.GetBestHeight()
	payload := GobEncode(Version{Version: version, BestHeight: bestHeight, AddrFrom: nodeAddress})
	request := append(CmdToBytes("version"), payload...)

	SendData(address, request)
}

// ============================================================================
// NETWORK MESSAGE HANDLERS
// ============================================================================

// HandleAddr processes incoming address lists from peers
func HandleAddr(request []byte) {
	var buff bytes.Buffer
	var payload Addr

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	// Add new nodes to our known nodes list
	KnownNodes = append(KnownNodes, payload.AddrList...)
	fmt.Printf("There are %d known nodes\n", len(KnownNodes))
	RequestBlocks() // Request blocks from new nodes
}

// HandleBlock processes incoming blocks and adds them to our blockchain
func HandleBlock(request []byte, chain *blockchain.Blockchain) {
	var buff bytes.Buffer
	var payload Block

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	blockData := payload.Block
	block := blockchain.Deserialize(blockData)

	fmt.Println("Received a new block!")
	chain.AddBlock(block)
	fmt.Printf("Added block %s to the chain\n", block.Hash)

	// If we have more blocks to download, request the next one
	if len(blocksInTransit) > 0 {
		blockHash := blocksInTransit[0]
		SendGetData(payload.AddrFrom, "block", blockHash)
		blocksInTransit = blocksInTransit[1:]
	} else {
		// All blocks downloaded, reindex UTXO set
		UTXOSet := blockchain.UTXOSet{Blockchain: chain}
		UTXOSet.Reindex()
	}
}

// HandleGetBlocks processes block hash requests and sends inventory
func HandleGetBlocks(request []byte, chain *blockchain.Blockchain) {
	var buff bytes.Buffer
	var payload GetBlocks

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	// Send inventory of all our block hashes
	blocks := chain.GetBlockHashes()
	SendInv(payload.AddrFrom, "block", blocks)
}

// HandleGetData processes requests for specific data (blocks or transactions)
func HandleGetData(request []byte, chain *blockchain.Blockchain) {
	var buff bytes.Buffer
	var payload GetData

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	// Send the requested block
	if payload.Type == "block" {
		block, err := chain.GetBlock([]byte(payload.ID))
		if err != nil {
			return // Block not found
		}
		SendBlock(payload.AddrFrom, &block)
	}

	// Send requested transaction from memory pool
	if payload.Type == "tx" {
		txID := hex.EncodeToString(payload.ID)
		tx := memoryPool[txID]
		SendTx(payload.AddrFrom, &tx)
	}
}

// HandleTx processes incoming transactions and adds them to the memory pool
func HandleTx(request []byte, chain *blockchain.Blockchain) {
	var buff bytes.Buffer
	var payload Tx

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	txData := payload.Transaction
	tx := blockchain.DeserializeTransaction(txData)

	// Add to the memory pool (unconfirmed transactions)
	memoryPool[hex.EncodeToString(tx.ID)] = tx
	fmt.Printf("%s, %d", nodeAddress, len(memoryPool))

	// If we're the central node, broadcast to all other nodes
	if nodeAddress == KnownNodes[0] {
		for _, node := range KnownNodes {
			if node != nodeAddress && node != payload.AddrFrom {
				SendInv(node, "tx", [][]byte{tx.ID})
			}
		}
	} else {
		// If we're a mining node and have enough transactions, mine a block
		if len(memoryPool) >= 2 && len(mineAddress) > 0 {
			MineTx(chain)
		}
	}
}

// MineTx mines a new block with transactions from the memory pool
func MineTx(chain *blockchain.Blockchain) {
	var txs []*blockchain.Transaction

	// Collect valid transactions from the memory pool
	for id := range memoryPool {
		fmt.Printf("Tx: %s\n", memoryPool[id].ID)
		tx := memoryPool[id]
		if chain.VerifyTransaction(&tx) {
			txs = append(txs, &tx)
		}
	}

	if len(txs) == 0 {
		fmt.Println("All Transactions are invalid")
		return
	}

	// Add coinbase transaction (mining reward)
	cbTx := blockchain.CoinbaseTx(mineAddress, "")
	txs = append(txs, cbTx)

	// Mine the new block
	newBlock := chain.MineBlock(txs)

	// Update UTXO set
	UTXOSet := blockchain.UTXOSet{Blockchain: chain}
	UTXOSet.Reindex()

	fmt.Println("New Block mined")

	// Remove mined transactions from the memory pool
	for _, tx := range txs {
		txID := hex.EncodeToString(tx.ID)
		delete(memoryPool, txID)
	}

	// Broadcast new block to network
	for _, node := range KnownNodes {
		if node != nodeAddress {
			SendInv(node, "block", [][]byte{newBlock.Hash})
		}
	}

	// If more transactions remain, continue mining
	if len(memoryPool) > 0 {
		MineTx(chain)
	}
}

// HandleVersion processes version messages during node handshake
func HandleVersion(request []byte, chain *blockchain.Blockchain) {
	var buff bytes.Buffer
	var payload Version

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	bestHeight := chain.GetBestHeight()
	otherHeight := payload.BestHeight

	// Determine who has a longer chain and sync accordingly
	if bestHeight < otherHeight {
		SendGetBlocks(payload.AddrFrom) // Request blocks if another node is ahead
	} else if bestHeight > otherHeight {
		SendVersion(payload.AddrFrom, chain) // Send our version if we're ahead
	}

	// Add a new node to known nodes if not already known
	if !NodeIsKnown(payload.AddrFrom) {
		KnownNodes = append(KnownNodes, payload.AddrFrom)
	}
}

// HandleInv processes inventory messages (advertisements of available data)
func HandleInv(request []byte, chain *blockchain.Blockchain) {
	var buff bytes.Buffer
	var payload Inv

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("Received inventory with %d %s\n", len(payload.Items), payload.Type)

	// Process block inventory
	if payload.Type == "block" {
		blocksInTransit = payload.Items

		// Request first block in inventory
		blockHash := payload.Items[0]
		SendGetData(payload.AddrFrom, "block", blockHash)

		// Remove the requested block from a transit list
		newInTransit := [][]byte{}
		for _, b := range blocksInTransit {
			if bytes.Compare(b, blockHash) != 0 {
				newInTransit = append(newInTransit, b)
			}
		}
		blocksInTransit = newInTransit
	}

	// Process transaction inventory
	if payload.Type == "tx" {
		txID := payload.Items[0]

		// Request transaction if we don't have it
		if memoryPool[hex.EncodeToString(txID)].ID == nil {
			SendGetData(payload.AddrFrom, "tx", txID)
		}
	}
}

// ============================================================================
// NETWORK SERVER & CONNECTION HANDLING
// ============================================================================

// HandleConnection processes incoming network connections
func HandleConnection(conn net.Conn, chain *blockchain.Blockchain) {
	req, err := ioutil.ReadAll(conn)
	defer conn.Close()

	if err != nil {
		log.Panic(err)
	}

	// Extract and process command
	command := BytesToCmd(req[:commandLength])
	fmt.Printf("Received %s command\n", command)

	// Route to the appropriate handler based on command
	switch command {
	case "addr":
		HandleAddr(req)
	case "block":
		HandleBlock(req, chain)
	case "inv":
		HandleInv(req, chain)
	case "getblocks":
		HandleGetBlocks(req, chain)
	case "getdata":
		HandleGetData(req, chain)
	case "tx":
		HandleTx(req, chain)
	case "version":
		HandleVersion(req, chain)
	default:
		fmt.Println("Unknown command")
	}
}

// GobEncode serializes data structures for network transmission
func GobEncode(data interface{}) []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

// NodeIsKnown checks if a node address is already in our known nodes list
func NodeIsKnown(addr string) bool {
	for _, node := range KnownNodes {
		if node == addr {
			return true
		}
	}
	return false
}

// CloseDB gracefully shuts down the database on process termination
func CloseDB(chain *blockchain.Blockchain) {
	d := death.NewDeath(syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	d.WaitForDeathWithFunc(func() {
		defer os.Exit(1)
		defer runtime.Goexit()
		chain.Database.Close()
	})
}

// ============================================================================
// MAIN NETWORK SERVER ENTRY POINT
// ============================================================================

// StartServer initializes and runs the P2P network node
// nodeID: Port number for this node (e.g., "3000", "3001")
// minerAddress: If not empty, this node will mine blocks to this address
func StartServer(nodeID, minerAddress string) {
	// Set the node address and mining address
	nodeAddress = fmt.Sprintf("localhost:%s", nodeID)
	mineAddress = minerAddress

	// Start listening for incoming connections
	ln, err := net.Listen(protocol, nodeAddress)
	if err != nil {
		log.Panic(err)
	}
	defer ln.Close()

	// Load or create a blockchain for this node
	chain := blockchain.ContinueBlockChain(nodeAddress)
	defer chain.Database.Close()

	// Set up a graceful shutdown
	go CloseDB(chain)

	// If this is the bootstrap node, broadcast our version
	if nodeAddress == KnownNodes[0] {
		SendVersion(nodeAddress, chain)
	}

	// Main server loop - accept and handle connections
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Panic(err)
		}
		go HandleConnection(conn, chain) // Handle in goroutine for concurrency
	}
}
