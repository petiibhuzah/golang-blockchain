# golang-blockchain

A minimal, educational blockchain implementation in Go. It demonstrates how blocks are linked using SHA‑256 hashes, how a genesis block is created, and how additional blocks are appended.

## Features
- Simple `Block` and `Blockchain` structures
- SHA‑256 based block hashing (`DeriveHash`)
- Genesis block creation
- Appending blocks with previous‑hash linkage

## Requirements
- Go 1.24+ (see `go.mod`)

## Getting Started
Clone this repository into your GOPATH (or anywhere if you use Go modules), then run:

```bash
go run main.go
```

## What the program does
The program:
1. Initializes a blockchain with a genesis block.
2. Appends three example blocks.
3. Prints each block's previous hash, data, and hash.

## Example output
Note: Your hashes will differ each run/machine due to data and environment differences. This is an example of the format you should see:

```
Previous Hash: 
Block Data: Genesis Block
Block Hash: 9f5d...

Previous Hash: 9f5d...
Block Data: First Block After Genesis
Block Hash: 3a1b...

Previous Hash: 3a1b...
Block Data: second Block After First Block
Block Hash: 7c92...

Previous Hash: 7c92...
Block Data: Third Block After Second Block
Block Hash: 4e0f...
```

## Execution result (screenshot)
The following image shows the actual program output captured after running `go run main.go`:

![Program execution output](execution.png)

If the image does not render in your environment, open `execution.png` in the project root.

## File overview
- `main.go` — Source code containing `Block`, `Blockchain`, and the `main` entrypoint
- `go.mod` — Go module definition
- `execution.png` — Screenshot of the program output
