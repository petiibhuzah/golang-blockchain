package main

import (
	"os"

	"github.com/golang-blockchain/cli"
)

/**
 * Created by GoLand.
 * Project: golang-blockchain
 * User: PETER DANIEL KILIMBA
 * Date: 26/11/2025
 * Time: 10:29
 */

func main() {
	defer os.Exit(0)
	cmd := cli.CommandLine{}
	cmd.Run()
}
