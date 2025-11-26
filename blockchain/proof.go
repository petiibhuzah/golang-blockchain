package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 26/11/2025
 * Time: 11:24
 */

/**
 * IMPLEMENTS THE PROOF OF WORK (PoW) CONSENSUS ALGORITHM
 *
 * Purpose:
 * This algorithm is fundamental to securing the blockchain. It prevents fraud and
 * ensures network consensus by making the process of adding a new block
 * computationally expensive (or "hard").
 *
 * How it Works:
 * Miners compete to solve a cryptographic puzzle that requires significant processing
 * power. The first miner to find a valid solution gets to add the block and is
 * rewarded, incentivizing participation.
 *
 * Key Principle:
 * - The "work" (mining) must be HARD to do.
 * - Verifying that the work was done correctly must be EASY for the rest of the network.
 *
 * This asymmetry is what makes PoW secure, as altering any past block would require
 * redoing all the subsequent work, which is computationally infeasible.
 */

// PROOF OF WORK MINING PROCESS
//
// Steps to mine a new block:
// 1. Extract block data (header, transactions, timestamp, previous hash)
// 2. Initialize nonce (number used once) to 0
// 3. Create candidate hash: hash(data + nonce)
// 4. Verify if the hash meets the current difficulty target
//    - Requirement: Hash must start with N leading zeros
//    - Example: "0000abc123..." for difficulty 4
// 5. If the requirement is not met:
//    - Increment nonce
//    - Return to step 3
// 6. If the requirement is met:
//    - Valid proof found! Block can be added to chain
//    - Miner receives a reward for work done

/* Proof of Work Miner
 * Iterates nonce to find hash meeting difficulty target (leading zeros).
 * Steps: 1) Get block data 2) Init nonce=0 3) Hash(data + nonce)
 *        4) Check leading zeros 5) Repeat until valid hash found.
 * Difficulty adjusts network-wide to control block generation time.
 */

/**
 * BITCOIN-STYLE PROOF OF WORK IMPLEMENTATION
 *
 * This implements the HashCash PoW system used in Bitcoin:
 *
 * PROCESS:
 * 1. Compose block header (version, prev_hash, merkle_root, timestamp, bits, nonce)
 * 2. Start with nonce = 0
 * 3. Calculate double-SHA256 hash of the entire header
 * 4. Check if hash < current target value (equivalent to leading zeros)
 * 5. If not valid, increment nonce and retry
 *
 * DIFFICULTY & TARGET:
 * - Requirement: Hash must be below a dynamic target value
 * - Expressed as: "First N bits must be zero"
 * - Originally 20 zero bits in HashCash, now regularly adjusted
 * - Higher difficulty = more leading zeros required = harder to mine
 *
 * SECURITY PROPERTIES:
 * - COMPUTATIONALLY HARD: Finding nonce requires brute force
 * - EASILY VERIFIABLE: Anyone can verify hash with one calculation
 * - ADJUSTABLE: Network difficulty changes to maintain 10min blocks
 */

/**
 * PROOF OF WORK DIFFICULTY SETTING
 *
 * Current: Static difficulty (for development/testing)
 * Real-world: Dynamic difficulty that adjusts periodically
 *
 * WHY DYNAMIC DIFFICULTY IS NEEDED:
 * - As more miners join the network, total computational power increases
 * - This would cause blocks to be produced too quickly without adjustment
 * - Dynamic difficulty maintains consistent block time (e.g., Bitcoin's 10-minute target)
 *
 * ADJUSTMENT MECHANISM:
 * - Regularly recalculated based on recent block production times
 * - If blocks are mined too fast → difficulty increases (more leading zeros required)
 * - If blocks are mined too slow → difficulty decreases (fewer leading zeros required)
 *
 * NOTE: This implementation uses fixed difficulty for simplicity.
 * In production, this would be replaced with a dynamic difficulty algorithm.
 */

const Difficulty = 12

type ProofOfWork struct {
	Block  *Block   // The block inside the blockchain
	Target *big.Int // The number that represents the requirements we described that derived by the difficulty. [The number to be targeted as nonce]
}

// NewProof Special function for taking the pointer from the block and produce the pointer to the proof of work
func NewProof(b *Block) *ProofOfWork {
	/*
			Decimal: 1
			Binary: 0000000000000000000000000000000000000000000000000000000000000001 (256 bits total)
		    Shift: ↑ 244 positions to the left
	*/
	target := big.NewInt(1)
	target.Lsh(target, uint(256-Difficulty))
	/*  In Binary:
			After: 000000000000 1 000000000000000000000000000000000000000000000000000
	               000000000000000000000000000000000000000000000000000000000000000000
	               000000000000000000000000000000000000000000000000000000000000000000
	               0000000000000000000000000000000000000000000000000
			         (12 zeros) ↑ then a 1, then 243 zeros
		2²⁴⁴ = 28,195,255,290,653,389,114,320,483,313,055,315,385,331,013,294,976,499,200,896,921,600
	    This would overflow with int64:
	*/

	//fmt.Println(target)        // Decimal: 28195255290653389114320483313055315385331013294976499200896921600
	//fmt.Printf("%b\n", target) // Binary: 1 followed by 244 zeros
	//fmt.Printf("%x\n", target) // Hexadecimal: 1 followed by 61 zeros
	pow := &ProofOfWork{b, target}
	return pow
}

// InitData Special function for creating the data to be hashed, replaces DeriveHash() in block.go
func (pow *ProofOfWork) InitData(nonce int) []byte {
	data := bytes.Join(
		[][]byte{
			pow.Block.PrevHash,       // Previous block's hash
			pow.Block.Data,           // Current block's transaction data
			ToHex(int64(nonce)),      // The nonce we're testing
			ToHex(int64(Difficulty)), // Current network difficulty
		},
		[]byte{}, // Separator (empty = no separator)
	)
	return data
}

// Run Special function for running our algorithm
func (pow *ProofOfWork) Run() (int, []byte) {
	var intHash big.Int
	var hash [32]byte

	nonce := 0
	// MINING LOOP: Iterate through possible nonce values
	// The nonce is the "number used once" that we change each iteration
	// to create different hash inputs until we find a valid proof
	for nonce < math.MaxInt64 {
		// 1. PREPARE DATA: Combine block data with current nonce
		//    This creates unique input for each mining attempt
		data := pow.InitData(nonce)

		// 2. CALCULATE HASH: Create SHA-256 hash of the data
		//    This is the computationally expensive part of Proof of Work
		hash = sha256.Sum256(data)

		// 3. DEBUG OUTPUT: Display a hash attempt (can be removed in production)
		//    Shows the mining progress in hexadecimal format
		fmt.Printf("\r%x", hash)

		// 4. CONVERT TO BIG INT: Convert hash to big.Int for mathematical comparison
		//    Allows us to compare the hash value against the target difficulty
		intHash.SetBytes(hash[:])

		// 5. VALIDITY CHECK: Test if hash meets target difficulty requirement
		//    Cmp returns -1 if hash < target, meaning valid proof found
		//    Target represents the maximum allowed hash value (with leading zeros)
		if intHash.Cmp(pow.Target) == -1 {
			// SUCCESS: Found a valid proof of work!
			// Hash has enough leading zeros to meet difficulty requirement
			break
		} else {
			// FAILURE: Hash doesn't meet difficulty requirement
			// Increment nonce and try again with different input
			nonce++
		}
	}
	fmt.Println()

	// RETURN: Valid nonce and corresponding hash
	// - nonce: The proof that work was done (must be included in block)
	// - hash: The valid hash that meets difficulty requirement
	return nonce, hash[:]
}

/**
 * VALIDATES PROOF OF WORK
 *
 * This function verifies that the work claimed in a block is actually valid.
 * It re-computes the hash using the block's stored nonce and checks if it
 * meets the difficulty target.
 *
 * Unlike Run() which does the computationally expensive work of finding a valid nonce,
 * this function only requires a single hash computation, making verification trivial.
 *
 * This demonstrates the core principle of Proof of Work:
 * - FINDING a valid proof is HARD (requires brute force)
 * - VERIFYING a valid proof is EASY (requires one calculation)
 */

func (pow *ProofOfWork) Validate() bool {
	var intHash big.Int

	// 1. RECONSTRUCT THE INPUT DATA
	// Use the nonce that was already found and stored in the block during mining
	// This recreates the exact same input that was used to create the valid proof
	data := pow.InitData(pow.Block.Nonce)

	// 2. COMPUTE THE HASH
	// Calculate SHA-256 hash of the data (single computation - very fast)
	// This is the same calculation that miners did millions of times during mining
	hash := sha256.Sum256(data)

	// 3. CONVERT TO BIG INT FOR COMPARISON
	// Convert the 32-byte hash to a big.Int for mathematical comparison
	intHash.SetBytes(hash[:])

	// 4. VERIFY AGAINST TARGET DIFFICULTY
	// Check if the computed hash is less than the target (has enough leading zeros)
	// Cmp returns -1 if hash < target, meaning the proof is valid
	return intHash.Cmp(pow.Target) == -1
}

/**
 * CONVERTS INT64 TO BIG-ENDIAN BYTE REPRESENTATION
 *
 * Despite the name "ToHex"; this function actually converts an int64 into its
 * 8-byte binary representation using big-endian byte order.
 *
 * WHAT IT DOES:
 * - Takes an int64 integer as input
 * - Converts it to exactly 8 bytes (64 bits) in big-endian format
 * - Returns the binary byte slice, NOT a hexadecimal string
 *
 * BIG-ENDIAN FORMAT:
 * - Most significant byte first (network standard)
 * - Example: int64(42) becomes []byte{0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x2A}
 * - Alternative (little-endian) would be: []byte{0x2A,0x00,0x00,0x00,0x00,0x00,0x00,0x00}
 *
 * USAGE IN PROOF OF WORK:
 * This function is used in InitData() to convert:
 * - Nonce values (int) → 8-byte binary for hashing
 * - Difficulty values (int) → 8-byte binary for hashing
 *
 * WHY BINARY/BIG-ENDIAN?
 * - Fixed size: All int64 values become exactly 8 bytes
 * - Consistent across different systems and architectures
 * - Efficient for cryptographic hashing operations
 * - Standard format for network protocols and blockchain systems
 *
 * EXAMPLE OUTPUTS:
 * ToHex(1)     → [0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x01]
 * ToHex(256)   → [0x00 0x00 0x00 0x00 0x00 0x00 0x01 0x00]
 * ToHex(65535) → [0x00 0x00 0x00 0x00 0x00 0x00 0xFF 0xFF]
 *
 * NOTE: The function name "ToHex" is misleading - it returns binary bytes,
 * not a hexadecimal string. A more accurate name would be "IntToBytes"
 * or "Int64ToBigEndian".
 */

func ToHex(num int64) []byte {
	buffer := new(bytes.Buffer)                        // Create a new bytes buffer
	err := binary.Write(buffer, binary.BigEndian, num) // Write num as binary bytes
	if err != nil {
		return nil
	}
	return buffer.Bytes() // Return the binary bytes
}
