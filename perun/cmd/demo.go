// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package cmd

import (
	"fmt"

	demo "perun.network/go-perun/perun/cmd/demo"

	prompt "github.com/c-bata/go-prompt"
	"github.com/spf13/cobra"
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Two party payment Demo",
	Long: `Enables two user to send payments between each other in a ledger state channel.
	The channels are funded and settled on an Ethereum blockchain, leaving out the dispute case.

	It illustrates how end user usage of Perun state channels can look like.`,
	Run: run,
}

func init() {
	rootCmd.AddCommand(demoCmd)
}

// run is executed everytime the program is started with the `demo` sub-command.
func run(c *cobra.Command, args []string) {
	demo.Setup()
	//flags := c.Flags()

	p := prompt.New(
		executor,
		completer,
		prompt.OptionPrefix("> "),
		prompt.OptionTitle("perun"),
	)
	p.Run()
}

func completer(prompt.Document) []prompt.Suggest {
	return []prompt.Suggest{}
}

// executor wraps the demo executor to print error messages.
func executor(in string) {
	if err := demo.Executor(in); err != nil {
		fmt.Println("\033[0;33m⚡\033[0m", err)
	}
}
