package wallet

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"math/big"

	"golang.org/x/crypto/ripemd160"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 04/12/2025
 * Time: 16:48
 */

// Wallet system constants
const (
	checksumLength = 4          // Length of checksum in bytes (used for error detection)
	version        = byte(0x00) // Network version byte (0x00 for Bitcoin mainnet)
	// The version byte is a critical identifier that tells the network which blockchain an address belongs to.
	// It's like an area code for cryptocurrencies.

	/*  MAINNET STANDS FOR MAIN NETWORK
	Common Version Bytes (Real World):
	Version	Network	                Address Prefix	      Example Address
	0x00	Bitcoin Mainnet	        Starts with 1	      1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa
	0x6F	Bitcoin Testnet	        Starts with m or n	  mipcBbFg9gMiCh81Kj8tqqdgoZub1ZJRfn
	0x05	Bitcoin Mainnet (P2SH)	Starts with 3	      3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy
	0xC4	Litecoin Mainnet	    Starts with L	      LbTjMGN7gELw4KbeyQf6cTCq859hD18guE
	0x30	Dogecoin Mainnet	    Starts with D	      D8dR8V8j5uV5t2v5BQ9vQ8r8z8Q8z8Q8z8
	0x1E	Dash Mainnet	        Starts with X	      XcY4WU5J8q8q8q8q8q8q8q8q8q8q8q8q8
	*/
)

// Wallet represents a cryptocurrency wallet containing cryptographic keys
// In blockchain, a wallet doesn't store coins - it stores keys to access them
type Wallet struct {
	PrivateKey ecdsa.PrivateKey // Private key for signing transactions (KEEP SECRET!)
	PublicKey  []byte           // Public key for verification (can be shared)
}

// Address generates a human-readable blockchain address from the wallet's public key
// This follows Bitcoin's address generation standard:
// PublicKey → SHA256 → RIPEMD160 → Add version → Add checksum → Base58Encode
func (w Wallet) Address() []byte {
	// Step 1: Create a public key hash (RIPEMD160(SHA256(public key)))
	pubHash := PublicKeyHash(w.PublicKey)

	// Step 2: Add version byte to identify network (0x00 = mainnet, 0x6f = testnet)
	versionedHash := append([]byte{version}, pubHash...)

	// Step 3: Calculate checksum for error detection (first 4 bytes of SHA256(SHA256(data)))
	checksum := Checksum(versionedHash)

	// Step 4: Combine version + pubHash + checksum
	fullHash := append(versionedHash, checksum...)

	// Step 5: Encode in Base58 for human readability (avoids similar-looking characters)
	address := Base58Encode(fullHash)
	return address
}

// Address: 14Vj9R8bVE1Rw97xM3LYHGF2Rbmvc82dh2
// FullHash: 001442540be4098f451a4d44204f3fd43895135f88
// [Version]: 00
// [PubKeyHash]: 42540be4098f451a4d44204f3fd43895135f88
// [Checksum]: mvc82dh2

// ValidateAddress checks if a cryptocurrency address is valid
// It verifies:
// 1. The address can be Base58 decoded
// 2. The structure is correct (version + pubkey hash + checksum)
// 3. The checksum matches the calculated checksum
func ValidateAddress(address string) bool {
	// Step 1: Decode the Base58 address back to binary
	// This gives us: [version(1)] + [pubKeyHash(20)] + [checksum(4)] = 25 bytes
	pubKeyHash := Base58Decode([]byte(address))

	// Validate length: Should be exactly 25 bytes for Bitcoin-style addresses
	// 1 byte version + 20 bytes hash + 4 bytes checksum = 25 bytes
	if len(pubKeyHash) != 25 {
		return false // Invalid length
	}

	// Step 2: Extract the parts
	addressVersion := pubKeyHash[0]       // First byte: network version
	pubKeyHashContent := pubKeyHash[1:21] // Next 20 bytes: actual hash
	actualChecksum := pubKeyHash[21:]     // Last 4 bytes: provided checksum

	// Step 3: Calculate what the checksum SHOULD be
	// Checksum is calculated from: a version + pubKeyHashContent
	payload := append([]byte{addressVersion}, pubKeyHashContent...)
	targetChecksum := Checksum(payload) // First 4 bytes of double SHA256

	// Step 4: Compare checksums
	// If they match, the address is valid (no typos)
	return bytes.Equal(actualChecksum, targetChecksum)
}

// NewKeyPair generates a new ECDSA key pair for cryptocurrency transactions
// Returns: private key (for signing) and public key (for verification)
func NewKeyPair() (ecdsa.PrivateKey, []byte) {
	// Use P-256 elliptic curve (also known as secp256r1 or prime256v1)
	// Bitcoin uses secp256k1, but P-256 is a common alternative for examples
	curve := elliptic.P256()

	// Generate private key using cryptographically secure random generator
	private, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Panic(err)
	}

	// Public key is concatenation of X and Y coordinates (uncompressed format)
	// In compressed format, you'd use X coordinate and parity bit
	publicKey := append(private.PublicKey.X.Bytes(), private.PublicKey.Y.Bytes()...)

	return *private, publicKey
}

// MakeWallet creates a new wallet with a fresh key pair
// This is the wallet constructor function
func MakeWallet() *Wallet {
	privateKey, publicKey := NewKeyPair()
	wallet := Wallet{privateKey, publicKey}
	return &wallet
}

// PublicKeyHash creates the public key hash using Bitcoin's standard method:
// SHA256 followed by RIPEMD160 (often called "Hash160")
func PublicKeyHash(pubKey []byte) []byte {
	// Step 1: SHA256 hash of the public key
	pubHash := sha256.Sum256(pubKey)

	// Step 2: RIPEMD160 hash of the SHA256 result
	// RIPEMD160 produces 160-bit (20-byte) output, shorter than SHA256
	hasher := ripemd160.New()
	_, err := hasher.Write(pubHash[:])
	if err != nil {
		log.Panic(err)
	}

	// Final RIPEMD160 hash (20 bytes)
	publicRipMD := hasher.Sum(nil)

	return publicRipMD
}

// Checksum calculates a 4-byte checksum using double SHA256
// Used for error detection in addresses (typos, transmission errors)
func Checksum(payload []byte) []byte {
	// First SHA256 hash
	firstHash := sha256.Sum256(payload)

	// Second SHA256 hash of the first hash
	secondHash := sha256.Sum256(firstHash[:])

	// Return the first 4 bytes (checksumLength) of the double hash
	return secondHash[:checksumLength]
}

// GobEncode implements gob.GobEncoder.
// We serialize only the private scalar D. Curve is fixed to P256, so we can
// reconstruct the full key from D when decoding.
func (w *Wallet) GobEncode() ([]byte, error) {
	// Just store D (private scalar) as bytes.
	data := struct {
		D []byte
	}{
		D: w.PrivateKey.D.Bytes(),
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(&data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode implements gob.GobDecoder.
// It restores the wallet by recreating the key on the P256 curve.
func (w *Wallet) GobDecode(b []byte) error {
	var data struct {
		D []byte
	}

	dec := gob.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&data); err != nil {
		return err
	}

	curve := elliptic.P256()

	d := new(big.Int).SetBytes(data.D)

	// Recompute public key (G * D)
	x, y := curve.ScalarBaseMult(data.D)

	priv := ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		},
		D: d,
	}

	w.PrivateKey = priv
	w.PublicKey = append(x.Bytes(), y.Bytes()...)

	return nil
}
