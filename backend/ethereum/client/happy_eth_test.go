// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package client_test

import (
	"context"
	"math/big"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/EthLaika/go-laika/ethclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/backend/ethereum/channel"
	"perun.network/go-perun/backend/ethereum/wallet"
	ethwallettest "perun.network/go-perun/backend/ethereum/wallet/test"
	clienttest "perun.network/go-perun/client/test"
	"perun.network/go-perun/log"
	"perun.network/go-perun/peer"
	peertest "perun.network/go-perun/peer/test"
	wallettest "perun.network/go-perun/wallet/test"
)

var defaultTimeout = 60 * time.Second

func TestHappyAliceBobETH(t *testing.T) {
	log.Info("Starting happy test")
	var hub peertest.ConnHub
	rng := rand.New(rand.NewSource(0x1337))
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Create a new KeyStore
	ks := ethwallettest.GetKeystore()
	// Create alice and bobs account
	aliceAcc := wallettest.NewRandomAccount(rng)
	bobAcc := wallettest.NewRandomAccount(rng)
	aliceAccETH := aliceAcc.(*wallet.Account).Account
	log.Infof("Creating Alice's account with address %s", aliceAccETH.Address.Hex())
	// eth.sendTransaction({from:'0x72981b6d3657593218d59b8113fa4d7c0f13b5f9', to:'0x191198Cd6e6699a1CF1d96dBe468288DDC90327B', value: web3.toWei(1, "ether"), gas:21000});
	bobAccETH := bobAcc.(*wallet.Account).Account
	log.Infof("Creating Bob's account with address %s", bobAccETH.Address.Hex())
	// eth.sendTransaction({from:'0x72981b6d3657593218d59b8113fa4d7c0f13b5f9', to:'0x4C6Fb6A04d28534382d165AA6A866e45c5a179b8', value: web3.toWei(1, "ether"), gas:21000});
	// Create SimulatedBackend
	//backend := test.NewSimulatedBackend()
	backend, err := ethclient.Dial("ws://localhost:8546")
	assert.NoError(t, err, "connecting to node failed")
	backendBob, err := ethclient.Dial("ws://localhost:8546")
	assert.NoError(t, err, "connecting to node failed")
	// Fund both accounts
	//backend.FundAddress(ctx, aliceAccETH.Address)
	//backend.FundAddress(ctx, bobAccETH.Address)
	// Create contract backends
	cbAlice := channel.NewContractBackend(backend, ks, aliceAccETH)
	cbBob := channel.NewContractBackend(backendBob, ks, bobAccETH)
	// Deploy the contracts
	adjAddr, err := channel.DeployAdjudicator(ctx, cbAlice)
	require.NoError(t, err, "Adjudicator should deploy successful")
	assetAddr, err := channel.DeployETHAssetholder(ctx, cbAlice, adjAddr)
	require.NoError(t, err, "ETHAssetholder should deploy successful")
	// Create the funders
	funderAlice := channel.NewETHFunder(cbAlice, assetAddr)
	funderBob := channel.NewETHFunder(cbBob, assetAddr)
	// Create the settlers
	settlerAlice := channel.NewETHSettler(cbAlice, adjAddr)
	settlerBob := channel.NewETHSettler(cbBob, adjAddr)

	setupAlice := clienttest.RoleSetup{
		Name:     "Alice",
		Identity: aliceAcc,
		Dialer:   hub.NewDialer(),
		Listener: hub.NewListener(aliceAcc.Address()),
		Funder:   funderAlice,
		Settler:  settlerAlice,
		Timeout:  defaultTimeout,
	}

	setupBob := clienttest.RoleSetup{
		Name:     "Bob",
		Identity: bobAcc,
		Dialer:   hub.NewDialer(),
		Listener: hub.NewListener(bobAcc.Address()),
		Funder:   funderBob,
		Settler:  settlerBob,
		Timeout:  defaultTimeout,
	}

	execConfig := clienttest.ExecConfig{
		PeerAddrs:       []peer.Address{aliceAcc.Address(), bobAcc.Address()},
		InitBals:        []*big.Int{big.NewInt(100), big.NewInt(100)},
		Asset:           &wallet.Address{Address: assetAddr},
		NumUpdatesBob:   2,
		NumUpdatesAlice: 2,
		TxAmountBob:     big.NewInt(5),
		TxAmountAlice:   big.NewInt(3),
	}

	alice := clienttest.NewAlice(setupAlice, t)
	bob := clienttest.NewBob(setupBob, t)
	// enable stages synchronization
	stages := alice.EnableStages()
	bob.SetStages(stages)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		log.Info("Starting Alice.Execute")
		alice.Execute(execConfig)
	}()

	go func() {
		defer wg.Done()
		log.Info("Starting Bob.Execute")
		bob.Execute(execConfig)
	}()

	wg.Wait()
	log.Info("Happy test done")
}
