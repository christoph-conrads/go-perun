// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package demo

import (
	"bytes"
	srand "crypto/rand"
	"encoding/hex"
	"math/big"
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"perun.network/go-perun/log"
	"perun.network/go-perun/wallet"
)

func valBal(input string) error {
	_, err := strconv.ParseInt(input, 10, 32)
	return errors.WithMessage(err, "Invalid Float")
}

func valString(input string) error {
	if len(input) < 1 {
		return errors.New("Invalid perun-id")
	}
	return nil
}

func valID(input string) error {
	if _, err := strToAddress(input); err != nil {
		return errors.New("Invalid perun-id")
	}
	return nil
}

func valIP(input string) error {
	if val := net.ParseIP(input); val == nil {
		return errors.New("Invalid IP")
	}
	return nil
}

func valUInt(input string) error {
	if n, err := strconv.Atoi(input); err != nil {
		return errors.New("Invalid integer")
	} else if n < 0 {
		return errors.New("Value must be > 0")
	}
	return nil
}

func valPeer(arg string) error {
	/*for alias := range node.peers {
		if alias == arg {
			return nil
		}
	}

	return errors.Errorf("unknown peer, ise 'info' to see available")*/
	return nil
}

func valSlice(strs []string) func(string) error {
	return func(str string) error {
		for _, s := range strs {
			if s == str {
				return nil
			}
		}
		return errors.Errorf("choose: %s", strings.Join(strs, ","))
	}
}

// strToAddress parses a string as wallet.Address
func strToAddress(str string) (wallet.Address, error) {
	h, err := hex.DecodeString(str[2:])
	if err != nil {
		return nil, errors.New("Could not parse address as hexadecimal")
	}
	addr, err := wallet.DecodeAddress(bytes.NewBuffer(h))
	return addr, errors.WithMessage(err, "string to address")
}

// nonce generates a cryptographically secure random value in the range [0, 2^256 -1]
func nonce() *big.Int {
	max := new(big.Int)
	max.Exp(big.NewInt(2), big.NewInt(256), nil).Sub(max, big.NewInt(1))

	val, err := srand.Int(srand.Reader, max)
	if err != nil {
		log.Panic("Could not create nonce")
	}
	return val
}
