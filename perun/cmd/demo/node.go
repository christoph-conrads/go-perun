// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package demo

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"perun.network/go-perun/apps/payment"
	_ "perun.network/go-perun/backend/ethereum" // backend init
	echannel "perun.network/go-perun/backend/ethereum/channel"
	ewallet "perun.network/go-perun/backend/ethereum/wallet"
	ewallettest "perun.network/go-perun/backend/ethereum/wallet/test"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/log"
	"perun.network/go-perun/peer/net"
	"perun.network/go-perun/wallet"

	"perun.network/go-perun/client"
	wtest "perun.network/go-perun/wallet/test"
)

type peer struct {
	alias   string
	perunID wallet.Address
	ch      *paymentChannel
}

type node struct {
	// rng is uses as substitution for a wallet to generate new accounts
	rng     *rand.Rand
	client  *client.Client
	log     log.Logger
	asset   channel.Asset
	acc     wallet.Account
	offAcc  wallet.Account
	dialer  *net.Dialer
	settler channel.Settler
	funder  channel.Funder
	cb      echannel.ContractBackend

	peers map[string]*peer
}

var backend *node

func newNode() (*node, error) {
	rng := rand.New(rand.NewSource(config.Seed))
	acc := wtest.NewRandomAccount(rng)
	log.WithField("on-chain", acc.Address()).Info()
	dialer := net.NewTCPDialer(config.Node.DialTimeout)
	backend, err := ethclient.Dial(config.Chain.URL)
	if err != nil {
		return nil, errors.WithMessage(err, "could not connect to ethereum node")
	}
	ethAcc := acc.(*ewallet.Account).Account
	ks := ewallettest.GetKeystore()
	cb := echannel.NewContractBackend(backend, ks, ethAcc) // will not be saved directly

	n := &node{
		rng:    rng,
		log:    log.Get(),
		acc:    acc,
		dialer: dialer,
		cb:     cb,
		peers:  make(map[string]*peer),
	}

	// Should this node deploy the contracts on its own, or use the given addresses from the config?
	if config.Chain.Adjudicator == "deploy" && config.Chain.Assetholder == "deploy" {
		if err := n.deploy(); err != nil {
			return nil, errors.WithMessage(err, "deploying contracts")
		}
	} else if config.Chain.Adjudicator == "deploy" || config.Chain.Assetholder == "deploy" {
		return nil, errors.New("Currently either both or none contract can be deployed")
	} else {
		adj, err := strToAddress(config.Chain.Adjudicator)
		if err != nil {
			return nil, err
		}
		ass, err := strToAddress(config.Chain.Assetholder)
		if err != nil {
			return nil, err
		}

		n.setContracts(adj.(*ewallet.Address).Address, ass.(*ewallet.Address).Address)
	}

	return n, n.listen()
}

func (n *node) Connect(args []string) error {
	n.log.Traceln("Connecting...")
	alias := args[2]
	// omit the 0x
	perunID, _ := strToAddress(args[1])

	if n.peers[alias] != nil {
		return errors.New("Peer exists already")
	}

	n.dialer.Register(perunID, args[0]+":"+strconv.Itoa(int(config.Node.OutPort)))
	ctx, cancel := context.WithTimeout(context.Background(), config.Node.DialTimeout)
	defer cancel()
	if _, err := n.dialer.Dial(ctx, perunID); err != nil {
		return errors.WithMessage(err, "could not connent to peer")
	}

	n.peers[alias] = &peer{
		alias:   alias,
		perunID: perunID,
	}

	return nil
}

func (n *node) deploy() error {
	ctxAdj, cancel := context.WithTimeout(context.Background(), config.Chain.AdjDeployTimeout)
	defer cancel()
	adjAddr, err := echannel.DeployAdjudicator(ctxAdj, n.cb)
	if err != nil {
		return errors.WithMessage(err, "deploying eth adjudicator")
	}

	ctxAss, cancel := context.WithTimeout(context.Background(), config.Chain.AssDeployTimeout)
	defer cancel()
	assAddr, err := echannel.DeployETHAssetholder(ctxAss, n.cb, adjAddr)
	if err != nil {
		return errors.WithMessage(err, "deploying eth assetholder")
	}

	n.setContracts(adjAddr, assAddr)
	return nil
}

func (n *node) setContracts(adjAddr, assAddr common.Address) {
	n.settler = echannel.NewETHSettler(n.cb, adjAddr)
	n.funder = echannel.NewETHFunder(n.cb, assAddr)
	n.asset = &ewallet.Address{Address: assAddr}
	n.log.WithField("Asset", adjAddr).WithField("Adj", assAddr).Debug("Set contracts")
}

func (n *node) listen() error {
	// here we simulate the generation of a new account from a wallet
	n.offAcc = wtest.NewRandomAccount(n.rng)
	n.log.WithField("off-chain", n.offAcc.Address()).Info("Generating account")

	n.client = client.New(n.acc, n.dialer, n, n.funder, n.settler)
	n.log.Trace("Created client object")
	host := config.Node.IP + ":" + strconv.Itoa(int(config.Node.InPort))
	n.log.WithField("host", host).Trace("Listening for connections")
	listener, err := net.NewTCPListener(host)
	if err != nil {
		return errors.WithMessage(err, "could not start tcp listener")
	}
	go n.client.Listen(listener)
	return nil
}

func (n *node) getPeer(addr wallet.Address) *peer {
	for _, peer := range n.peers {
		if peer.perunID == addr {
			return peer
		}
	}
	return nil
}

var aliasCounter int

func nextAlias() string {
	alias := fmt.Sprintf("peer%d", aliasCounter)
	aliasCounter++
	return alias
}

func (n *node) Handle(req *client.ChannelProposalReq, res *client.ProposalResponder) {
	from := req.PeerAddrs[1]
	n.log.WithField("from", from).Debug("Channel propsal")

	// Find the peer by its perunID and create it if not present
	p := n.getPeer(from)
	if p == nil {
		alias := nextAlias()
		p = &peer{
			alias:   alias,
			perunID: from,
		}
		n.peers[alias] = p
		n.log.WithField("id", from).WithField("alias", alias).Debug("New peer")
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Node.HandleTimeout)
	defer cancel()

	_ch, err := res.Accept(ctx, client.ProposalAcc{
		Participant: n.offAcc,
	})
	if err != nil {
		n.log.Error(errors.WithMessage(err, "accepting channel proposal"))
		return
	}

	// Add the channel to the peer and start listening for updates
	p.ch = newPaymentChannel(_ch)
	go p.ch.ListenUpdates()
}

func (n *node) Open(args []string) error {
	if n.client == nil {
		return errors.New("Please 'deploy' first")
	}
	peer := n.peers[args[0]]
	if peer == nil {
		return errors.Errorf("peer not found %s", args[0])
	}
	myBals, _ := strconv.ParseInt(args[1], 10, 32) // error already checked by validator
	peerBals, _ := strconv.ParseInt(args[2], 10, 32)

	initBals := &channel.Allocation{
		Assets: []channel.Asset{n.asset},
		OfParts: [][]*big.Int{
			{big.NewInt(myBals)},
			{big.NewInt(peerBals)},
		},
	}
	prop := &client.ChannelProposal{
		ChallengeDuration: config.Channel.ChallengeDurationSec,
		Nonce:             nonce(),
		Account:           n.offAcc,
		AppDef:            payment.AppDef(),
		InitData:          new(payment.NoData),
		InitBals:          initBals,
		PeerAddrs:         []wallet.Address{n.acc.Address(), peer.perunID},
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Channel.FundTimeout)
	defer cancel()
	_ch, err := n.client.ProposeChannel(ctx, prop)
	if err != nil {
		return errors.WithMessage(err, "proposing channel failed")
	}

	peer.ch = newPaymentChannel(_ch)
	go peer.ch.ListenUpdates()

	return nil
}

func (n *node) Send(args []string) error {
	n.log.Traceln("Sending...")
	peer := n.peers[args[0]]
	if peer == nil {
		return errors.Errorf("peer not found %s", args[0])
	}
	howMuch, _ := strconv.ParseInt(args[1], 10, 32)
	peer.ch.sendMoney(big.NewInt(howMuch))

	return nil
}

func (n *node) Close(args []string) error {
	n.log.Traceln("Closing...")
	alias := args[0]
	peer := n.peers[alias]
	if peer == nil {
		return errors.Errorf("Unknown peer: %s", alias)
	}
	if err := peer.ch.sendFinal(); err != nil {
		return errors.WithMessage(err, "sending final state for state closing")
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Channel.SettleTimeout)
	defer cancel()
	if err := peer.ch.Settle(ctx); err != nil {
		return errors.WithMessage(err, "settling the channel")
	}

	if err := peer.ch.Close(); err != nil {
		return errors.WithMessage(err, "channel closing")
	}
	peer.ch.log.Debug("Removing channel")
	delete(n.peers, alias)

	return nil
}

// Info prints the phase of all channels.
func (n *node) Info(args []string) error {
	n.log.Traceln("Info...")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.Debug)
	fmt.Fprintf(w, "Peer\tPhase\tMy Ξ\tPeer Ξ\t\n")
	for alias, peer := range n.peers {
		fmt.Fprintf(w, "%s\t", alias)
		if peer.ch == nil {
			fmt.Fprintf(w, "Connected\t\n")
		} else {
			my, other := peer.ch.GetBalances()
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n", alias, peer.ch.Phase().String(), my.String(), other.String())
		}
	}
	fmt.Fprintln(w)
	w.Flush()

	return nil
}

func (n *node) Exit([]string) error {
	n.log.Traceln("Exiting...")

	return n.client.Close()
}

// Setup initializes the node, can not be done init() since it needs the configuration
// from viper.
func Setup() {
	SetConfig()
	rng := rand.New(rand.NewSource(0x280a0f350eec))
	appDef := wtest.NewRandomAddress(rng)
	payment.SetAppDef(appDef)

	b, err := newNode()
	if err != nil {
		log.Fatalln("init error:", err)
	}
	backend = b
}
