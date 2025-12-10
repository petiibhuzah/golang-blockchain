# golang-blockchain

An educational blockchain in Go that demonstrates blocks, Proof‑of‑Work mining, UTXO‑based transactions, on‑disk persistence with BadgerDB, and a basic wallet system with ECDSA keys and Base58Check addresses. This project is intended for learning and small experiments — not for production use.

## Features
- Core data structures: `Block`, `Transaction`, and `Blockchain`
- UTXO transaction model (inputs/outputs) including coinbase rewards
- SHA‑256 hashing and block linkage
- Merkle tree over per‑block transactions (Merkle root used by PoW)
- Proof of Work miner with validation
- Persistent storage via BadgerDB (on-disk key-value store)
- Simple CLI: create chain, send, get balance, print chain
- Gob-based serialization of blocks and transactions
- Wallets: ECDSA keypairs, Base58Check addresses, on‑disk wallet store, CLI to create/list addresses
- Digital signatures: ECDSA (P‑256) signing of transaction inputs and full verification on validation

## Requirements
- Go 1.24+ (see `go.mod`)

## Quick start (CLI)
Initialize a chain with a coinbase to your address, send funds, check balances, and print the chain.

```bash
# 0) Create a wallet and note the generated address
go run main.go createwallet
go run main.go listaddresses

# 1) Create a new blockchain and mine the genesis coinbase to your address
# Replace ADDRESS with one of the Base58 addresses printed above
go run main.go createblockchain -address ADDRESS

# 2) Check balance (reads UTXOs)
go run main.go getbalance -address ADDRESS

# 3) Send coins (creates a transaction, mined into a new block)
# Requires two wallet addresses
go run main.go send -from ADDRESS1 -to ADDRESS2 -amount 25

# 4) Query balances
go run main.go getbalance -address ADDRESS1
go run main.go getbalance -address ADDRESS2

# 5) Print the full chain from tip back to genesis
go run main.go printchain
```

Notes
- The blockchain is persisted under `./tmp/blocks` in the repo root.
- `createblockchain` creates a new DB and mines a genesis block that contains a coinbase transaction paying the specified address.
- When you run `send`, the transaction’s inputs are signed automatically with the sender’s private key from the local wallet; nodes verify these signatures before accepting the transaction/block.

## What the program does
1. Creates or opens a BadgerDB-backed blockchain with a genesis coinbase to the provided address.
2. Builds UTXO transactions from inputs and outputs when you `send` value from one address to another.
3. Mines new blocks via Proof of Work until a valid hash under the target is found.
4. Persists blocks and allows reverse iteration from the tip to genesis.
5. Prints each block and validates its PoW.

## BadgerDB persistence (highlight)
- Database path: `./tmp/blocks` (both `Dir` and `ValueDir`).
- Keys and values:
  - `"lh"` → bytes of the last block’s hash (tip pointer).
  - `block.Hash` → serialized `Block` (Gob-encoded bytes, including transactions).
- UTXO set index (see section below) → keys prefixed with `"utxo-"` store serialized unspent outputs per transaction.
- Transactions:
  - Read-only: `View` to fetch values (e.g., current last hash, block by hash).
  - Read-write: `Update` to store new blocks and advance `"lh"`.
- Serialization:
  - `block.Serialize()` encodes a `Block` (with its `Transactions`) using `encoding/gob` before writing.
  - `Deserialize(data)` decodes bytes back into a `Block` when reading.
- Cleanup:
  - The DB handle is closed on exit (`Database.Close()` in `main.go`).

### UTXO set persisted in BadgerDB (fast unspent lookups)
This project maintains a persistent UTXO (Unspent Transaction Output) index in BadgerDB to avoid rescanning the entire blockchain when building new transactions or checking balances.

- File: `blockchain/utxo.go`
- Keyspace: keys are prefixed with `utxo-` followed by the transaction ID (`[]byte`).
- Value: a serialized `TxOutputs` structure containing all currently unspent outputs of that transaction.
- Core operations:
  - `UTXOSet.Reindex()` — rebuilds the UTXO index from the full chain (useful after a reset or for first-time build).
  - `UTXOSet.Update(block)` — incrementally updates the index when a new block is mined: it removes spent outputs and adds any new outputs created by transactions in the block.
  - `UTXOSet.FindSpendableOutputs(pubKeyHash, amount)` — coin selection for building transactions; returns enough unspent outputs to cover `amount`.
  - `UTXOSet.FindUnspentTransactions(pubKeyHash)` — lists all unspent outputs for an address (used to compute balances).

Benefits
- Constant-time iteration over just the UTXO keyspace (Badger iterator with `utxo-` prefix) rather than scanning all blocks.
- Faster balance queries and transaction creation, since only unspent outputs are read.
- Durable across runs because the UTXO set is stored on disk alongside blocks under `./tmp/blocks`.

### Merkle tree (transaction root per block)
Each block computes a Merkle root over its transactions and uses that root as part of the Proof‑of‑Work input. This provides a single cryptographic fingerprint of all transactions in the block.

- File: `blockchain/merkle.go`
- API used: `(*Block).HashTransactions()` builds the Merkle tree from each transaction’s serialized bytes and returns `tree.RootNode.Data`.
- Leaf hashing: `SHA‑256(tx.Serialize())`.
- Internal node hashing: `SHA‑256(leftHash || rightHash)`.
- Odd number of leaves: duplicates the last leaf to make the count even (common convention in blockchains).

Why it matters
- Integrity: any change to any transaction changes the Merkle root and thus the mined block hash.
- Efficient proofs: enables Merkle proofs (not implemented here, but the structure supports it) for verifying inclusion without all transactions.

Resetting the chain
- Stop the program and delete the `./tmp/blocks` directory to start fresh:

```bash
rm -rf ./tmp/blocks
```

## CLI usage
```
Usage:
 getbalance -address ADDRESS - get the balance of an address
 createblockchain -address ADDRESS - create a blockchain
 printchain - Print the blocks in the chain
 send -from FROM -to TO -amount AMOUNT - Send coins from one address to another
 createwallet - Create a new wallet
 listaddresses - Lists the addresses in the wallet file
```

Examples
```bash
go run main.go createwallet
go run main.go listaddresses
# Copy two addresses, then:
go run main.go createblockchain -address ADDRESS1
go run main.go getbalance -address ADDRESS1
go run main.go send -from ADDRESS1 -to ADDRESS2 -amount 25
go run main.go printchain
```

## Wallets: keys, addresses, and persistence
The `wallet` package introduces a minimal wallet system built on real cryptography and Bitcoin‑style addressing.

Files
- `wallet/wallet.go` — ECDSA keypair generation, address derivation, custom Gob encoding/decoding of keys
- `wallet/utils.go` — Base58 encode/decode helpers
- `wallet/wallets.go` — Multi‑wallet manager, persistence to disk, create/list/get wallets
- `wallet.mmd` — Mermaid diagram of the wallet flows (documentation)

Key generation
- Uses ECDSA on the P‑256 curve to generate `PrivateKey` and `PublicKey` (`X||Y` bytes)
- `MakeWallet()` creates a new wallet with a fresh keypair

Address derivation (Bitcoin‑style)
1. `SHA‑256(public key)`
2. `RIPEMD‑160` of the result → 20‑byte public key hash
3. Prepend version byte `0x00` (mainnet‑like prefix)
4. Compute checksum = first 4 bytes of `SHA‑256(SHA‑256(version||pkh))`
5. Concatenate and `Base58` encode → human‑readable address

Persistence
- All wallets (address → keypair) are stored in `./tmp/wallet.data` using `encoding/gob`
- `Wallets.SaveFile()` and `Wallets.LoadFile()` handle serialization/deserialization

CLI integration
- `createwallet` — generates a new wallet and persists it; prints the new address
- `listaddresses` — prints all known addresses from `./tmp/wallet.data`

Security notes
- The private key is stored locally; treat `./tmp/wallet.data` as sensitive
- Back up this file if you want to keep access to your funds in future runs
- This project is for education; do not use these keys/addresses on real networks

### Impact on the project
- Address format: The system now uses Base58Check addresses derived from ECDSA keys, rather than arbitrary strings.
- Transactions and balances: CLI commands `send` and `getbalance` accept these addresses. UTXO lookup continues to be keyed by address string.
- Persistence: A new data file `./tmp/wallet.data` is introduced in addition to the BadgerDB directory `./tmp/blocks`.
- New CLI surface: `createwallet` and `listaddresses` were added to manage local addresses used by the chain.
- Serialization: Wallets use Gob encoding. Blocks/transactions are unaffected by this change and continue to be Gob‑encoded as before.

## Architecture overview
### Data structures
- Block (`blockchain/block.go`)
  - `Hash []byte`: hash of the current block
  - `Transactions []*Transaction`: list of transactions included in this block
  - `PrevHash []byte`: hash of the previous block
  - `Nonce int`: value found by mining (set during PoW)
- Blockchain (`blockchain/blockchain.go`)
  - `LastHash []byte`: hash of the tip (for iteration and adding new blocks)
  - `Database *badger.DB`: BadgerDB instance holding all blocks
- Iterator (`blockchain/blockchain.go`)
  - Supports reverse traversal from tip to genesis using stored hashes

### Transactions & UTXO model
- Transaction (`blockchain/transaction.go`)
  - `ID []byte`: unique transaction hash
  - `Inputs []TxInput`: references to previously unspent outputs being spent now
  - `Outputs []TxOutput`: newly created outputs (who can spend the value next)
- TxInput
  - `ID []byte`: transaction ID of the referenced output
  - `Out int`: index within that transaction’s outputs
  - `Signature []byte`: ECDSA signature over a deterministic hash of the transaction (per‑input)
  - `PubKey []byte`: full uncompressed public key (`X||Y`) of the spender (used for verification)
- TxOutput
  - `Value int`: amount
  - `PubKeyHash []byte`: 20‑byte public key hash the output is locked to (derived from an address)

Helper/constructor functions
- `CoinbaseTx(to, data string) *Transaction`: mines reward to `to` in genesis and as first tx of mined blocks
- `NewTransaction(from, to string, amount int, chain *Blockchain) *Transaction`: builds a transaction by gathering spendable UTXOs, creating change if needed, and setting the transaction ID
- `(*Blockchain).FindUTXO(address string) []TxOutput`: scans the chain to collect unspent outputs for an address
- `(*Blockchain).FindSpendableOutputs(address string, amount int) (acc int, validOutputs map[string][]int)`: selects sufficient UTXOs to cover an amount

### Hashing and linkage
- Transactions inside a block are organized in a Merkle tree. Leaves are `SHA‑256` of each transaction’s serialized bytes; internal nodes hash the concatenation of child hashes. The resulting Merkle root from `(*Block).HashTransactions()` is used as part of the PoW input.
- The PoW miner (`NewProof.Run`) finds a valid nonce and sets `block.Hash` directly from the mined hash.

### Block creation and addition
- `Genesis(coinbaseTx)` creates the first block with a coinbase transaction.
- `CreateBlock(txs, prevHash)` constructs a block (with transactions) and runs PoW to fill `Nonce` and `Hash`.
- `(*Blockchain).AddBlock(transactions)` mines a block with the provided transactions and persists it to BadgerDB, updating the last-hash pointer `"lh"`.

## Proof of Work (concise)
- Difficulty constant in `blockchain/proof.go` (e.g., `const Difficulty = 20`).
- Target: `1 << (256 - Difficulty)`; valid block hash must be less than this target.
- Mining loop: increment `Nonce`, compute SHA‑256, compare to target, repeat until valid.
- Validation: `Validate()` recomputes using the stored `Nonce` and checks against the target.

Adjusting difficulty
```go
const Difficulty = 20
```
- Increase → harder (slower mining)
- Decrease → easier (faster mining)

## Example output
Your hashes will differ per run/machine. Below is a screenshot from a representative run showing CLI execution and PoW validation.

![Execution results](execution.png)

Tip: To display the nonce, ensure printing in `printchain` includes `block.Nonce` if desired.

## Project layout
- `blockchain/block.go` — Block type, serialization helpers
- `blockchain/blockchain.go` — BadgerDB-backed blockchain, iterator, (legacy) UTXO scanning
- `blockchain/proof.go` — Difficulty, target building, mining, validation
- `blockchain/transaction.go` — Transactions, inputs/outputs, coinbase, digital signatures (sign/verify), builders
- `blockchain/merkle.go` — Merkle tree construction for per‑block transaction root
- `blockchain/utxo.go` — Persistent UTXO set index (BadgerDB `utxo-` keys) for fast balances and coin selection
- `main.go` — CLI entrypoint (`createblockchain`, `getbalance`, `send`, `printchain`)
- `execution.png` — Example output screenshot
- `go.mod`, `go.sum` — Go module and dependencies (includes BadgerDB v4)
- `wallet/wallet.go`, `wallet/utils.go`, `wallet/wallets.go` — Wallets, addresses, persistence; `wallet.mmd` for docs

## Digital signatures in transactions
This project now uses real digital signatures for spending UTXOs, modeled after Bitcoin’s approach but simplified.

What is signed
- Each input is signed separately using ECDSA on the P‑256 curve.
- The data being signed is a deterministic hash of a trimmed copy of the transaction:
  - All inputs have `Signature=nil` and `PubKey=nil` in the copy.
  - For the input currently being signed, its `PubKey` field is temporarily set to the `PubKeyHash` of the referenced output.
  - The transaction copy is then hashed (`txCopy.Hash()`), and that hash is signed.

How signing works
- `(*Transaction).Sign(privateKey, prevTXs)`:
  - Skips coinbase transactions (they don’t spend previous outputs).
  - Builds the trimmed copy, iterates inputs, prepares per‑input context, computes `txCopy.ID`, and calls `ecdsa.Sign`.
  - The resulting `r||s` bytes are stored in `TxInput.Signature` of the original transaction.

How verification works
- `(*Transaction).Verify(prevTXs) bool`:
  - Skips coinbase transactions (always valid).
  - Recreates the exact same trimmed copy and per‑input context as in signing.
  - Splits `Signature` into `r` and `s`, reconstructs the public key from `TxInput.PubKey`, then calls `ecdsa.Verify` using the computed hash.
  - All inputs must verify for the transaction to be valid.

Developer references
- Signing: `blockchain/transaction.go` → `Transaction.Sign`
- Verification: `blockchain/transaction.go` → `Transaction.Verify`
- Deterministic copy: `blockchain/transaction.go` → `Transaction.TrimmedCopy`

CLI notes
- `send` automatically loads the sender’s wallet, builds a transaction, signs all inputs, and broadcasts/mines it.
- During block validation and when scanning UTXOs, signatures are verified to ensure only rightful owners can spend outputs.

## Limitations and learning notes
- No networking, mempool, or consensus beyond local PoW
- Simplified transaction locking/unlocking. Wallets use real ECDSA keys for address derivation only; transactions still use simplified fields.
- Fixed difficulty; no dynamic retargeting
- Simple persistence model (single process; no compaction controls beyond Badger defaults)

## Troubleshooting
- Mining appears slow: lower `Difficulty` in `blockchain/proof.go` for faster demos.
- Reset the chain: delete `./tmp/blocks` and rerun to recreate genesis.
- Closing the DB: ensure the program exits normally, or explicitly close `chain.Database` in your own code.

Wallet tips
- If `listaddresses` shows nothing on a fresh repo, run `createwallet` first — the wallet file is created lazily.
- If you delete `./tmp/wallet.data`, you will lose access to previously generated addresses and any funds sent to them in your local chain.
