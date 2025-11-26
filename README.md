# golang-blockchain

A minimal, educational blockchain in Go that shows end-to-end block creation, chaining, and validation, with a working Proof of Work (PoW) miner. This repository is intended for learning and small experiments — not for production use.

## Features
- Core data structures: simple `Block` and `Blockchain`
- Hashing and block linkage using SHA‑256
- Genesis block creation and sequential block appending
- Optional mining via Proof of Work (configurable difficulty)
- PoW target calculation using big integers (`2^(256 - Difficulty)`)
- Nonce search loop and PoW validation

## Requirements
- Go 1.24+ (see `go.mod`)

## Quick start
Run the demo program:

```bash
go run main.go
```

You will see transient hashes while the miner searches for a valid nonce. When a valid proof is found, the block is appended and printed.

## What the program does
1. Initializes a blockchain with a genesis block.
2. Appends three demo blocks with example data.
3. Mines each new block using Proof of Work until a hash under the target is found.
4. Prints each block’s previous hash, data, final hash, and whether `PoW` validation passes.

## Architecture overview
### Data structures
- Block
  - `Hash []byte`: hash of the current block
  - `Data []byte`: payload (transaction/record/demo text)
  - `PrevHash []byte`: hash of the previous block
  - `Nonce int`: proof value found by mining (set during PoW)
- Blockchain
  - `Blocks []*Block`: slice of blocks, ordered from genesis to tip

### Hashing and linkage
- Basic hashing (method `DeriveHash`) demonstrates how to hash `Data || PrevHash` with SHA‑256.
- In the current flow, mining (`ProofOfWork.Run`) finds a valid nonce and sets `block.Hash` directly from the mined hash; `DeriveHash` serves as a didactic example.

### Block creation and addition
- `Genesis()` creates the first block.
- `CreateBlock(data, prevHash)` constructs a block and runs PoW to fill `Nonce` and `Hash`.
- `AddBlock(data)` fetches the previous block, builds a new block, mines it, and appends it to the chain.

## Proof of Work (concise)
- Difficulty: constant `Difficulty = 12` in `blockchain/proof.go`.
- Target: `1 << (256 - Difficulty)`; a valid block hash must be less than this target.
- Data to hash: `PrevHash || Data || Nonce || Difficulty` (see `InitData`).
- Mining loop: increment `Nonce`, compute SHA‑256, compare to target, repeat until valid.
- Validation: `Validate()` recomputes the hash using the stored `Nonce` and checks it against the target.

### Adjusting difficulty
Change the constant in `blockchain/proof.go`:

```
const Difficulty = 12
```

- Increase → harder (slower mining, more leading zeros required)
- Decrease → easier (faster mining)

Note: Difficulty is fixed for simplicity. Real networks retarget based on recent block times.

## Example output
Your hashes will differ on each run/machine. Below is a representative snippet of what `main.go` prints today:

```
Previous Hash: 
Block Data: Genesis Block
Hash: 9f5d...

Previous Hash: 9f5d...
Block Data: First Block After Genesis
Hash: 003a1b...
PoW: true

Previous Hash: 003a1b...
Block Data: second Block After Genesis
Hash: 0007c92...
PoW: true

Previous Hash: 0007c92...
Block Data: Third Block After Genesis
Hash: 004e0f...
PoW: true
```

Tip: The mined `Nonce` is stored on each block but is not printed by default. If you want to display it, you can add `fmt.Printf("Nonce: %d\n", block.Nonce)` in `main.go`.

## Using as a tiny library
You can construct and extend a chain from your own code:

```
chain := blockchain.InitBlockChain()
chain.AddBlock("My first block")
for _, b := range chain.Blocks {
    fmt.Printf("%x -> %s\n", b.Hash, string(b.Data))
}
```

## Project layout
- `blockchain/block.go` — Block/Blockchain types and block creation with PoW
- `blockchain/proof.go` — Difficulty, target building, mining, validation
- `main.go` — Demo program / entrypoint
- `execution.png` — Screenshot of program output
- `go.mod` — Go module definition

## Limitations and learning notes
- No networking, mempool, or consensus beyond local PoW
- No persistent storage; the chain exists in-memory only
- No transactions, UTXOs, or signatures; `Data` is free-form bytes
- Fixed difficulty; no dynamic retargeting

## Roadmap (ideas)
- Print additional block fields (e.g., Nonce) in the demo
- Add serialization/persistence to disk
- Introduce basic transactions and signing
- Add simple CLI (add block, print chain, verify)

## Troubleshooting
- Mining appears stuck: with higher difficulty, it may take time; reduce `Difficulty` for faster demos.
- No output image: open `execution.png` directly from the repo root.
