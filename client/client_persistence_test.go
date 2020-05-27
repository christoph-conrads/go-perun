// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package client_test

import (
	"math/big"
	"math/rand"
	"testing"

	chprtest "perun.network/go-perun/channel/persistence/test"
	chtest "perun.network/go-perun/channel/test"
	ctest "perun.network/go-perun/client/test"
	"perun.network/go-perun/peer"
	ptest "perun.network/go-perun/peer/test"
)

func TestPersistencePetraRobert(t *testing.T) {
	rng := rand.New(rand.NewSource(0x70707))
	setups, _hub := NewSetupsPersistence(t, rng, []string{"Petra", "Robert"})
	hub := (*connHub)(_hub)
	roles := [2]ctest.Executer{
		ctest.NewPetra(setups[0], hub, t),
		ctest.NewRobert(setups[1], hub, t),
	}

	cfg := ctest.ExecConfig{
		PeerAddrs:  [2]peer.Address{setups[0].Identity.Address(), setups[1].Identity.Address()},
		Asset:      chtest.NewRandomAsset(rng),
		InitBals:   [2]*big.Int{big.NewInt(100), big.NewInt(100)},
		NumUpdates: [2]int{2, 2},
		TxAmounts:  [2]*big.Int{big.NewInt(5), big.NewInt(3)},
	}

	executeTwoPartyTest(t, roles, cfg)
}

type connHub ptest.ConnHub // wrapper for correct return type signatures

func (h *connHub) NewListener(addr peer.Address) peer.Listener {
	return (*ptest.ConnHub)(h).NewListener(addr)
}
func (h *connHub) NewDialer() peer.Dialer { return (*ptest.ConnHub)(h).NewDialer() }

func NewSetupsPersistence(t *testing.T, rng *rand.Rand, names []string) ([]ctest.RoleSetup, *ptest.ConnHub) {
	setups, hub := NewSetups(rng, names)
	for i := range names {
		setups[i].PR = chprtest.NewPersistRestorer(t)
	}
	return setups, hub
}
