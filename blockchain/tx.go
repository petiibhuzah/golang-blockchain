package blockchain

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 04/12/2025
 * Time: 16:45
 */

// TxOutput represents an indivisible unit of value that can be spent
// Think of it like a "check" or "voucher" that can be redeemed
type TxOutput struct {
	Value  int    // Number of tokens/coins being transferred
	PubKey string // Locking condition: address that can spend this output
}

// TxInput references a previous output that is being spent
// It's like presenting a check stub to cash it
type TxInput struct {
	ID        []byte // Transaction ID containing the output being spent
	Out       int    // Index of the output in the previous transaction
	Signature string // The Proof that the spender owns the output (unlock signature)
}

// CanUnlock determines if someone can spend this transaction input
//
// In a real blockchain, this would verify a cryptographic signature proving
// ownership of the referenced output. This simplified demo uses basic string
// comparison instead of actual signature verification.
func (in *TxInput) CanUnlock(unlockingData string) bool {
	// SIMPLIFIED VERSION: Just compares strings
	// REAL VERSION: Would verify digital signature using public key cryptography
	// Example real check: VerifySignature(in.Signature, unlockingData, in.ID)

	return in.Signature == unlockingData
}

// CanBeUnlocked determines who can spend this transaction output in the future
//
// Returns true if the provided data (typically an address or public key)
// matches the output's locking condition (PubKey field).
func (out *TxOutput) CanBeUnlocked(unlockingData string) bool {
	// Checks if this output is locked to the provided address/key
	// If true, the holder of the matching private key can spend this output later

	return out.PubKey == unlockingData
}
