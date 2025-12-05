# golang-blockchain

An educational blockchain in Go that demonstrates blocks, Proof‑of‑Work mining, UTXO‑based transactions, on‑disk persistence with BadgerDB, and a basic wallet system with ECDSA keys and Base58Check addresses. This project is intended for learning and small experiments — not for production use.

## Features
- Core data structures: `Block`, `Transaction`, and `Blockchain`
- UTXO transaction model (inputs/outputs) including coinbase rewards
- SHA‑256 hashing and block linkage
- Proof of Work miner with validation
- Persistent storage via BadgerDB (on-disk key-value store)
- Simple CLI: create chain, send, get balance, print chain
- Gob-based serialization of blocks and transactions
- Wallets: ECDSA keypairs, Base58Check addresses, on‑disk wallet store, CLI to create/list addresses

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
- Transactions:
  - Read-only: `View` to fetch values (e.g., current last hash, block by hash).
  - Read-write: `Update` to store new blocks and advance `"lh"`.
- Serialization:
  - `block.Serialize()` encodes a `Block` (with its `Transactions`) using `encoding/gob` before writing.
  - `Deserialize(data)` decodes bytes back into a `Block` when reading.
- Cleanup:
  - The DB handle is closed on exit (`Database.Close()` in `main.go`).

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
  - `Signature string`: simplified unlock data (demo; not real cryptography)
- TxOutput
  - `Value int`: amount
  - `PubKey string`: simplified lock to an address

Helper/constructor functions
- `CoinbaseTx(to, data string) *Transaction`: mines reward to `to` in genesis and as first tx of mined blocks
- `NewTransaction(from, to string, amount int, chain *Blockchain) *Transaction`: builds a transaction by gathering spendable UTXOs, creating change if needed, and setting the transaction ID
- `(*Blockchain).FindUTXO(address string) []TxOutput`: scans the chain to collect unspent outputs for an address
- `(*Blockchain).FindSpendableOutputs(address string, amount int) (acc int, validOutputs map[string][]int)`: selects sufficient UTXOs to cover an amount

### Hashing and linkage
- Transactions inside a block are hashed (joined) to produce a deterministic transaction root for PoW input via `(*Block).HashTransactions()`.
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
- `blockchain/blockchain.go` — BadgerDB-backed blockchain, iterator, UTXO scanning
- `blockchain/proof.go` — Difficulty, target building, mining, validation
- `blockchain/transaction.go` — Transactions, inputs/outputs, coinbase, builders
- `main.go` — CLI entrypoint (`createblockchain`, `getbalance`, `send`, `printchain`)
- `execution.png` — Example output screenshot
- `go.mod`, `go.sum` — Go module and dependencies (includes BadgerDB v4)
- `wallet/wallet.go`, `wallet/utils.go`, `wallet/wallets.go` — Wallets, addresses, persistence; `wallet.mmd` for docs

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
