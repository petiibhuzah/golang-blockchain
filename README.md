# golang-blockchain

An educational blockchain in Go that demonstrates block creation, proof-of-work mining, and on-disk persistence using BadgerDB. This project is intended for learning and small experiments — not for production use.

## Features
- Core data structures: `Block` and `Blockchain`
- SHA‑256 hashing and block linkage
- Proof of Work miner with validation
- Persistent storage via BadgerDB (on-disk key-value store)
- Simple CLI: add blocks and print the chain
- Gob-based serialization of blocks

## Requirements
- Go 1.24+ (see `go.mod`)

## Quick start (CLI)
Initialize or open the chain, add blocks, and print them.

```bash
# Add a new block with data
go run main.go add -block "Hello, Blockchain"

# Print the full chain from tip back to genesis
go run main.go print
```

Notes
- The blockchain is persisted under `.tmp/blocks` in the repo root.
- The database is created automatically on first run (a genesis block is also created if none exists).

## What the program does
1. Initializes (or opens) a BadgerDB-backed blockchain and ensures a genesis block exists.
2. Allows adding new blocks via CLI (`add -block "..."`).
3. Mines each new block using Proof of Work until a valid hash under the target is found.
4. Prints each block’s previous hash, data, final hash, and whether PoW validation passes.

## BadgerDB persistence (highlight)
- Database path: `.tmp/blocks` (both `Dir` and `ValueDir`).
- Keys and values:
  - `"lh"` → bytes of the last block’s hash (tip pointer).
  - `block.Hash` → serialized `Block` (Gob-encoded bytes).
- Transactions:
  - Read-only: `View` to fetch values (e.g., current last hash, block by hash).
  - Read-write: `Update` to store new blocks and advance `"lh"`.
- Serialization:
  - `block.Serialize()` encodes a `Block` with `encoding/gob` before writing.
  - `Deserialize(data)` decodes bytes back into a `Block` when reading.
- Cleanup:
  - The DB handle is closed on exit (`Database.Close()` in `main.go`).

Resetting the chain
- Stop the program and delete the `.tmp/blocks` directory to start fresh:

```bash
rm -rf .tmp/blocks
```

## CLI usage
```
Usage:
 add - block BLOCK_DATA - add a block to the chain
 print - Prints the blocks in the chain
```

Examples
```bash
go run main.go add -block "First block after genesis"
go run main.go add -block "Second block"
go run main.go print
```

## Architecture overview
### Data structures
- Block (`blockchain/block.go`)
  - `Hash []byte`: hash of the current block
  - `Data []byte`: payload (transaction/record/demo text)
  - `PrevHash []byte`: hash of the previous block
  - `Nonce int`: value found by mining (set during PoW)
- Blockchain (`blockchain/blockchain.go`)
  - `LastHash []byte`: hash of the tip (for iteration and adding new blocks)
  - `Database *badger.DB`: BadgerDB instance holding all blocks
- Iterator (`blockchain/blockchain.go`)
  - Supports reverse traversal from tip to genesis using stored hashes

### Hashing and linkage
- `DeriveHash` shows basic hashing of `Data || PrevHash` with SHA‑256.
- The PoW miner (`NewProof.Run`) finds a valid nonce and sets `block.Hash` directly from the mined hash.

### Block creation and addition
- `Genesis()` creates the first block (if none exists).
- `CreateBlock(data, prevHash)` constructs a block and runs PoW to fill `Nonce` and `Hash`.
- `(*Blockchain).AddBlock(data)` mines a block and persists it to BadgerDB, updating the last-hash pointer `"lh"`.

## Proof of Work (concise)
- Difficulty constant in `blockchain/proof.go` (e.g., `const Difficulty = 12`).
- Target: `1 << (256 - Difficulty)`; valid block hash must be less than this target.
- Mining loop: increment `Nonce`, compute SHA‑256, compare to target, repeat until valid.
- Validation: `Validate()` recomputes using the stored `Nonce` and checks against the target.

Adjusting difficulty
```go
const Difficulty = 12
```
- Increase → harder (slower mining)
- Decrease → easier (faster mining)

## Example output
Your hashes will differ per run/machine. A representative snippet:

```
Previous Hash: 
Block Data: Genesis Block
Hash: 9f5d...

Previous Hash: 9f5d...
Block Data: First block after genesis
Hash: 003a1b...
PoW: true
```

Tip: To display the nonce, add `fmt.Printf("Nonce: %d\n", block.Nonce)` in `main.go` when printing.

## Using as a tiny library
Construct and extend a chain programmatically. Since persistence is via BadgerDB, use the iterator to traverse blocks:

```go
chain := blockchain.InitBlockChain()
defer chain.Database.Close()

chain.AddBlock("My first block")

iter := chain.Iterator()
for {
    b := iter.Next()
    fmt.Printf("%x -> %s\n", b.Hash, string(b.Data))
    if len(b.PrevHash) == 0 {
        break // reached genesis
    }
}
```

## Project layout
- `blockchain/block.go` — Block type, serialization helpers
- `blockchain/blockchain.go` — BadgerDB-backed blockchain, iterator
- `blockchain/proof.go` — Difficulty, target building, mining, validation
- `main.go` — CLI entrypoint (`add`, `print`)
- `execution.png` — Example output screenshot
- `go.mod`, `go.sum` — Go module and dependencies (includes BadgerDB v4)

## Limitations and learning notes
- No networking, mempool, or consensus beyond local PoW
- No transactions, UTXOs, or signatures; `Data` is free-form bytes
- Fixed difficulty; no dynamic retargeting
- Simple persistence model (single process; no compaction controls beyond Badger defaults)

## Troubleshooting
- Mining appears slow: lower `Difficulty` in `blockchain/proof.go` for faster demos.
- Reset the chain: delete `.tmp/blocks` and rerun to recreate genesis.
- Closing the DB: ensure the program exits normally, or explicitly close `chain.Database` in your own code.
