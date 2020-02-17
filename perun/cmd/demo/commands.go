// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package demo

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"perun.network/go-perun/log"
)

type argument struct {
	Name      string
	Validator func(string) error
}

type command struct {
	Name     string
	Args     []argument
	Help     string
	Function func([]string) error
}

var commands []command

func init() {
	commands = []command{
		{
			"connect",
			[]argument{{"IP", valIP}, {"perun-id", valString}, {"Alias", valString}},
			"Connect to peer with Perun ID perun-id at host host (ip or hostname) at port port and call the peer alias as a shortcut. Port can be omitted if default port is used.\nExample: connect 192.168.0.22:530300 0x12ef... alice",
			func(args []string) error { return backend.Connect(args) },
		}, {
			"open",
			[]argument{{"Alias", valPeer}, {"Our Balance", valBal}, {"Their Balance", valBal}},
			"Open a payment channel with the given peer. It is only possible to open one channel per peer. A configurable default timeout will be used.\nExample: open alice 10 10",
			func(args []string) error { return backend.Open(args) },
		}, {
			"send",
			[]argument{{"Alias", valPeer}, {"Amount", valBal}},
			"Send a payment with amount to a given peer over the established channel.\nExample: send alice 5",
			func(args []string) error { return backend.Send(args) },
		}, {
			"close",
			[]argument{{"Alias", valPeer}},
			"Close a channel with the given peer and print the final balances. It should send a finalization request, conclude the final state and withdraw.\nExample: close alice",
			func(args []string) error { return backend.Close(args) },
		}, {
			"info",
			nil,
			//[]argument{{"What to inform you off", valSlice([]string{"channel", "peer"})}},
			"Info",
			func(args []string) error { return backend.Info(args) },
		}, {
			"help",
			nil,
			"Prints all possible commands",
			printHelp,
		}, {
			"exit",
			nil,
			"Exits the program",
			func(args []string) error {
				if err := backend.Exit(args); err != nil {
					log.Error("err while exiting: ", err)
				}
				os.Exit(0)
				return nil
			},
		},
	}
}

// Executor interprets commands entered by the user.
// Gets called by Cobra, but could also be used for emulating user input.
func Executor(in string) error {
	in = strings.TrimSpace(in)
	args := strings.Split(in, " ")
	command := args[0]
	args = args[1:]

	for _, cmd := range commands {
		if cmd.Name == command {
			if len(args) != len(cmd.Args) {
				return errors.Errorf("Invalid number of arguments, expected %d but got %d", len(cmd.Args), len(args))
			}
			for i, arg := range args {
				if err := cmd.Args[i].Validator(arg); err != nil {
					return errors.WithMessagef(err, "'%s' argument invalid for '%s': %v", cmd.Args[i].Name, command, arg)
				}
			}
			return cmd.Function(args)
		}
	}
	return errors.Errorf("Unknown command: %s", command)
}

func printHelp(args []string) error {
	for _, cmd := range commands {
		fmt.Print(cmd.Name, " ")
		for _, arg := range cmd.Args {
			fmt.Printf("<%s> ", arg.Name)
		}
		fmt.Printf("\n\t%s\n\n", strings.ReplaceAll(cmd.Help, "\n", "\n\t"))
	}

	return nil
}
