package wallet

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 04/12/2025
 * Time: 17:13
 */

import (
	"log"

	"github.com/mr-tron/base58"
)

// Base58Encode converts binary data to a Base58-encoded string
// Base58 is used in cryptocurrencies for human-friendly addresses
// It avoids ambiguous characters that look similar
func Base58Encode(input []byte) []byte {
	// Encode the raw bytes to Base58 string
	// Example: []byte{0x00, 0x01} → "1"
	encode := base58.Encode(input)

	// Convert the Base58 string back to []byte for consistency
	// This may seem odd, but it maintains []byte return type
	// Caller can convert to string if needed: string(encoded)
	return []byte(encode)
}

// Base58Decode converts a Base58-encoded string back to original binary data
// This is the inverse operation of Base58Encode
func Base58Decode(input []byte) []byte {
	// input is []byte containing Base58 characters
	// Convert to string for the decode function
	// Example: "1" → []byte{0x00, 0x01}
	decode, err := base58.Decode(string(input[:]))

	// If decoding fails, panic (in production, you'd handle this gracefully)
	// Common failures: invalid Base58 characters, checksum mismatch
	if err != nil {
		log.Panic(err)
	}

	// Return the original binary data
	return decode
}

// Base58 Character Set: WHY THESE CHARACTERS ARE ELIMINATED
// ----------------------------------------------------------
// Base58 removes 6 confusing characters from Base64:
//
// 0 (zero)     vs   O (capital o)  → Look similar
// I (capital i) vs  l (lowercase L) → Look similar
// + (plus)     vs   / (slash)      → Look similar in some fonts
//
// This prevents human errors when reading/copying addresses
