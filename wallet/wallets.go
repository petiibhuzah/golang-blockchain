package wallet

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 05/12/2025
 * Time: 12:53
 */

// walletFile defines the persistent storage location for wallet data
// This file stores all wallets in serialized format for persistence across restarts
const walletFile = "./tmp/wallets_%s.data"

// Wallets is a collection of cryptocurrency wallets
// It manages multiple wallet instances, each with its own key pair and address
type Wallets struct {
	// Map of address -> Wallet pointer
	// Address (string) is the Base58-encoded public address
	// Wallet contains the private/public key pair for that address
	Wallets map[string]*Wallet
}

// CreateWallets initializes a wallet collection and loads existing wallets from disk
// This is the factory function for creating/loading a wallet manager
func CreateWallets(nodeID string) (*Wallets, error) {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet) // Initialize empty map

	// Attempt to load existing wallets from a file
	// If a file doesn't exist, returns an empty wallet collection
	err := wallets.LoadFile(nodeID)
	return &wallets, err
}

// AddWallet creates a new wallet, adds it to the collection, and returns its address
// This generates a fresh key pair - each call creates a new, unique wallet
func (ws *Wallets) AddWallet() string {
	// Generate a new cryptographic key pair
	wallet := MakeWallet() // Creates private/public keys

	// Get the Base58-encoded address from the wallet
	// The address is derived from the public key
	address := fmt.Sprintf("%s", wallet.Address()) // Convert []byte to string

	// Store the wallet in the map using address as a key
	ws.Wallets[address] = wallet

	// Persist to disk to prevent data loss
	ws.SaveFile(address)

	// Return the new address so the caller knows what was created
	return address
}

// GetAllAddresses returns a list of all wallet addresses in the collection
// Useful for displaying available wallets or iterating through them
func (ws *Wallets) GetAllAddresses() []string {
	addresses := make([]string, 0) // Initialize an empty slice

	// Iterate through map keys (addresses)
	for address := range ws.Wallets {
		addresses = append(addresses, address)
	}

	return addresses // Returns something like: ["1A1zP...", "1B2qP..."]
}

// GetWallet retrieves a specific wallet by its address
// Returns nil if the address doesn't exist in the collection
func (ws *Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address] // Map lookup - O(1) complexity
}

// LoadFile reads wallet data from disk and deserializes it
// This restores the wallet state from a previous session
func (ws *Wallets) LoadFile(nodeID string) error {
	filePath := fmt.Sprintf(walletFile, nodeID)
	// Check if a wallet file exists
	// If not, return an error (a file will be created on the first SaveFile)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return err // File doesn't exist yet (first run)
	}

	var wallets Wallets

	// Read the entire file into memory
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err // Can't read file (permissions, corrupted, etc.)
	}

	// Create decoder to deserialize the binary data
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))

	// Decode the binary data back into Wallets struct
	err = decoder.Decode(&wallets)
	if err != nil {
		return err // Corrupted or incompatible data
	}

	// Copy the loaded wallets into the current instance
	ws.Wallets = wallets.Wallets

	return nil
}

// SaveFile serializes all wallets to disk for persistence
// This should be called whenever wallets are modified
func (ws *Wallets) SaveFile(nodeID string) {
	var content bytes.Buffer // Buffer to hold serialized data
	filePath := fmt.Sprintf(walletFile, nodeID)

	// Create an encoder to serialize to binary format
	encoder := gob.NewEncoder(&content)

	// Serialize the entire Wallets struct (including the map)
	err := encoder.Encode(ws)
	if err != nil {
		log.Panic(err) // Should never happen unless data is corrupted
	}

	// Write serialized data to a file with user read/write permissions
	// 0644 = an owner can read/write, others can read
	err = ioutil.WriteFile(filePath, content.Bytes(), 0644)
	if err != nil {
		log.Panic(err) // Disk full, permissions, etc.
	}
}
