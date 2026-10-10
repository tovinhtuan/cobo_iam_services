// Command bcrypthash reads one password per stdin line and prints one bcrypt hash per line.
// Used by scripts/devqa/provision_qa_accounts.py so passwords never appear on a command line.
package main

import (
	"bufio"
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	in := bufio.NewScanner(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for in.Scan() {
		hash, err := bcrypt.GenerateFromPassword(in.Bytes(), bcrypt.DefaultCost)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bcrypt:", err)
			os.Exit(1)
		}
		fmt.Fprintln(out, string(hash))
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}
}
