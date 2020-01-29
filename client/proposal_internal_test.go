// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package client

import (
	"context"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	channeltest "perun.network/go-perun/channel/test"
	"perun.network/go-perun/peer"
	peertest "perun.network/go-perun/peer/test"
	"perun.network/go-perun/wallet"
	wallettest "perun.network/go-perun/wallet/test"
)

type doNotUseDialer struct {
	t *testing.T
}

func (d *doNotUseDialer) Dial(context.Context, peer.Address) (peer.Conn, error) {
	d.t.Error("the dialer must not be called")
	return nil, errors.New("doNotUseDialer.Dial() must not be called")
}

func (d *doNotUseDialer) Close() error {
	return errors.New("doNotUseDialer.Close() must not be called")
}

func TestClient_ProposeChannel_InvalidProposal(t *testing.T) {
	rng := rand.New(rand.NewSource(0x20200123a))
	account := wallettest.NewRandomAccount(rng)
	proposal := newRandomValidChannelProposalReq(rng, 2).AsProp(account)
	invalidProposal := proposal
	invalidProposal.ChallengeDuration = 0
	c := New(
		wallettest.NewRandomAccount(rng),
		&doNotUseDialer{t},
		new(DummyProposalHandler),
		new(DummyFunder),
		new(DummySettler),
	)

	_, err := c.ProposeChannel(context.Background(), invalidProposal)
	assert.Error(t, err)
}

// SimpleProposalHandler calls a given callback whenever it is invoked as a
// handler and after callback execution, the channel returned by Done() is
// closed.
type SimpleProposalHandler struct {
	callback func(*ChannelProposalReq, *ProposalResponder)
	// The proposal handler may be executed concurrently. This channel allows
	// on to check if the handler finished execution.
	done chan struct{}
}

var _ ProposalHandler = (*SimpleProposalHandler)(nil)

func NewSimpleHandler(
	f func(*ChannelProposalReq, *ProposalResponder)) *SimpleProposalHandler {
	return &SimpleProposalHandler{f, make(chan struct{})}
}

func (h *SimpleProposalHandler) Handle(
	proposal *ChannelProposalReq, responder *ProposalResponder) {
	defer close(h.done)
	h.callback(proposal, responder)
}

func (h *SimpleProposalHandler) Done() <-chan struct{} {
	return h.done
}

func TestClient_exchangeTwoPartyProposal(t *testing.T) {
	rng := rand.New(rand.NewSource(0x20200123b))
	timeout := time.Duration(1 * time.Second)
	connHub := new(peertest.ConnHub)
	client0 := New(
		wallettest.NewRandomAccount(rng),
		connHub.NewDialer(),
		new(DummyProposalHandler),
		new(DummyFunder),
		new(DummySettler),
	)
	defer client0.Close()

	makeProposal := func(rng *rand.Rand, peerAddress wallet.Address) *ChannelProposalReq {
		proposal := newRandomValidChannelProposalReq(rng, 2)
		proposal.PeerAddrs[0] = client0.id.Address()
		proposal.PeerAddrs[1] = peerAddress
		return proposal
	}
	makeClient := func(rng *rand.Rand, proposalHandler ProposalHandler) *Client {
		return New(
			wallettest.NewRandomAccount(rng),
			connHub.NewDialer(),
			proposalHandler,
			new(DummyFunder),
			new(DummySettler),
		)
	}

	// In the test cases below, as soon as the test case finished,
	// * contexts are cancelled,
	// * case-specific clients are closed (named `client1`).
	//
	// For this reason, you *must* wait for the proposal handler to
	// finish before exiting a test case *if* the proposal handler sends or
	// receives data. Otherwise, the send/recv call may return an error due to
	// a cancelled context or closed clients/peers/dialers.

	t.Run("accept-proposal", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		partAccount := wallettest.NewRandomAccount(rng)
		callback := func(proposal *ChannelProposalReq, responder *ProposalResponder) {
			assert.NoError(t, proposal.Valid())

			msgAccept := &ChannelProposalAcc{
				SessID:          proposal.SessID(),
				ParticipantAddr: partAccount.Address(),
			}
			assert.NoError(t, responder.peer.Send(ctx, msgAccept))
		}
		proposalHandler := NewSimpleHandler(callback)
		client1 := makeClient(rng, proposalHandler)
		defer client1.Close()

		listener := connHub.NewListener(client1.id.Address())
		go client1.Listen(listener)

		proposal := makeProposal(rng, client1.id.Address())
		addresses, err := client0.exchangeTwoPartyProposal(ctx, proposal)
		assert.NoError(t, err)
		require.Equal(t, len(proposal.PeerAddrs), len(addresses))
		assert.Equal(t, proposal.ParticipantAddr, addresses[0])
		assert.Equal(t, partAccount.Address(), addresses[1])

		<-proposalHandler.done
	})

	t.Run("reject-proposal", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		callback := func(proposal *ChannelProposalReq, responder *ProposalResponder) {
			assert.NoError(t, proposal.Valid())
			assert.NoError(t, responder.Reject(ctx, "rejection reason"))
		}
		proposalHandler := NewSimpleHandler(callback)
		client1 := makeClient(rng, proposalHandler)
		defer client1.Close()

		listener := connHub.NewListener(client1.id.Address())
		go client1.Listen(listener)

		proposal := makeProposal(rng, client1.id.Address())
		addresses, err := client0.exchangeTwoPartyProposal(ctx, proposal)
		assert.Nil(t, addresses)
		assert.Error(t, err)

		<-proposalHandler.done
	})

	t.Run("accept-proposal-invalid-sid", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		partAccount := wallettest.NewRandomAccount(rng)
		callback := func(proposal *ChannelProposalReq, responder *ProposalResponder) {
			assert.NoError(t, proposal.Valid())

			invalidSessID := proposal.SessID()
			invalidSessID[0] = ^invalidSessID[0]
			msgAccept := &ChannelProposalAcc{
				SessID:          invalidSessID,
				ParticipantAddr: partAccount.Address(),
			}
			assert.NoError(t, responder.peer.Send(ctx, msgAccept))
		}
		proposalHandler := NewSimpleHandler(callback)
		client1 := makeClient(rng, proposalHandler)
		defer client1.Close()

		listener := connHub.NewListener(client1.id.Address())
		go client1.Listen(listener)

		// this test will cause a timeout of `exchangeTwoPartyProposal` (if it
		// is implemented properly). use a shorter timeout to avoid long wait
		// times.
		ctxShort, cancelShort := context.WithTimeout(
			context.Background(), 100*time.Millisecond)
		defer cancelShort()
		proposal := makeProposal(rng, client1.id.Address())
		addresses, err := client0.exchangeTwoPartyProposal(ctxShort, proposal)
		assert.Nil(t, addresses)
		assert.Error(t, err)

		<-proposalHandler.done
	})

	t.Run("reject-proposal-invalid-sid", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		callback := func(proposal *ChannelProposalReq, responder *ProposalResponder) {
			assert.NoError(t, proposal.Valid())

			invalidSessID := proposal.SessID()
			invalidSessID[0] = ^invalidSessID[0]
			msgReject := &ChannelProposalRej{
				SessID: invalidSessID,
				Reason: "reject-proposal-invalid-sid-reason",
			}
			assert.NoError(t, responder.peer.Send(ctx, msgReject))
		}
		proposalHandler := NewSimpleHandler(callback)
		client1 := makeClient(rng, proposalHandler)
		defer client1.Close()

		listener := connHub.NewListener(client1.id.Address())
		go client1.Listen(listener)

		// this test will cause a timeout of `exchangeTwoPartyProposal` (if it
		// is implemented properly). use a shorter timeout to avoid long wait
		// times.
		ctxShort, cancelShort := context.WithTimeout(
			context.Background(), 100*time.Millisecond)
		defer cancelShort()
		proposal := makeProposal(rng, client1.id.Address())
		addresses, err := client0.exchangeTwoPartyProposal(ctxShort, proposal)
		assert.Nil(t, addresses)
		assert.Error(t, err)

		<-proposalHandler.done
	})

	t.Run("connection-close-after-sending-proposal", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		callback := func(proposal *ChannelProposalReq, responder *ProposalResponder) {
			assert.NoError(t, proposal.Valid())
			assert.NoError(t, responder.peer.Close())
		}
		proposalHandler := NewSimpleHandler(callback)
		client1 := makeClient(rng, proposalHandler)
		defer client1.Close()

		listener := connHub.NewListener(client1.id.Address())
		go client1.Listen(listener)

		proposal := makeProposal(rng, client1.id.Address())
		addresses, err := client0.exchangeTwoPartyProposal(ctx, proposal)
		assert.Nil(t, addresses)
		assert.Error(t, err)
	})

	t.Run("context-timeout-before-sending-proposal", func(t *testing.T) {
		client1 := makeClient(
			rng,
			NewSimpleHandler(func(_ *ChannelProposalReq, _ *ProposalResponder) {
				assert.Fail(t, "proposal handler should not be called")
			}),
		)
		defer client1.Close()

		listener := connHub.NewListener(client1.id.Address())
		go client1.Listen(listener)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cancel()
		proposal := makeProposal(rng, client1.id.Address())
		addresses, err := client0.exchangeTwoPartyProposal(ctx, proposal)
		assert.Nil(t, addresses)
		assert.Error(t, err)
	})

	t.Run("context-timeout-after-sending-proposal", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		callback := func(proposal *ChannelProposalReq, responder *ProposalResponder) {
			assert.NoError(t, proposal.Valid())
			cancel()
		}
		proposalHandler := NewSimpleHandler(callback)
		client1 := makeClient(rng, proposalHandler)
		defer client1.Close()

		listener := connHub.NewListener(client1.id.Address())
		go client1.Listen(listener)

		proposal := makeProposal(rng, client1.id.Address())
		addresses, err := client0.exchangeTwoPartyProposal(ctx, proposal)
		assert.Nil(t, addresses)
		assert.Error(t, err)
	})

}

func TestClient_handleChannelProposal(t *testing.T) {
	rng := rand.New(rand.NewSource(0x20200128))
	timeout := time.Duration(1 * time.Second)
	connHub := new(peertest.ConnHub)
	// the following instance of client.Client is called a server because all
	// other instances of Client will connect to it
	server := New(
		wallettest.NewRandomAccount(rng),
		connHub.NewDialer(),
		new(DummyProposalHandler),
		new(DummyFunder),
		new(DummySettler),
	)
	defer server.Close()

	listener := connHub.NewListener(server.id.Address())
	go server.Listen(listener)

	newProposal := func(rng *rand.Rand, peerAddress wallet.Address) *ChannelProposalReq {
		proposal := newRandomValidChannelProposalReq(rng, 2)
		proposal.PeerAddrs[0] = server.id.Address()
		proposal.PeerAddrs[1] = peerAddress
		return proposal
	}
	newClient := func(rng *rand.Rand, proposalHandler ProposalHandler) *Client {
		return New(
			wallettest.NewRandomAccount(rng),
			connHub.NewDialer(),
			proposalHandler,
			new(DummyFunder),
			new(DummySettler),
		)
	}

	t.Run("no-handler-call-with-invalid-proposal", func(t *testing.T) {
		client := newClient(
			rng,
			NewSimpleHandler(func(_ *ChannelProposalReq, _ *ProposalResponder) {
				assert.Fail(t, "the proposal handler should not be called")
			}),
		)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		peer, err := client.peers.Get(ctx, server.id.Address())
		assert.NotNil(t, peer)
		assert.NoError(t, err)

		invalidProposal := newProposal(rng, client.id.Address())
		invalidProposal.ChallengeDuration = 0
		client.handleChannelProposal(peer, invalidProposal)
		// the proposal handler signals an error if it is called
	})

	t.Run("check-handler-is-called", func(t *testing.T) {
		proposalHandler := NewSimpleHandler(
			func(_ *ChannelProposalReq, _ *ProposalResponder) {})
		client := newClient(rng, proposalHandler)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		proposal := newProposal(rng, client.id.Address())
		peer, err := client.peers.Get(ctx, server.id.Address())
		assert.NotNil(t, peer)
		assert.NoError(t, err)
		assert.NoError(t, proposal.Valid())
		client.handleChannelProposal(peer, proposal)

		select {
		case <-proposalHandler.Done():
		case <-ctx.Done():
			assert.Fail(t, "proposal handler was not called before timeout")
		}
	})

	t.Run("handler-repeatedly-accepting-proposal", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		callback := func(proposal *ChannelProposalReq, responder *ProposalResponder) {
			assert.NoError(t, proposal.Valid())

			id0 := wallettest.NewRandomAccount(rng)
			responder.Accept(ctx, ProposalAcc{id0})

			id1 := wallettest.NewRandomAccount(rng)
			assert.Panics(t, func() { responder.Accept(ctx, ProposalAcc{id1}) })
		}
		proposalHandler := NewSimpleHandler(callback)
		client := newClient(rng, proposalHandler)
		proposal := newProposal(rng, client.id.Address())
		peer, err := client.peers.Get(ctx, server.id.Address())
		assert.NotNil(t, peer)
		assert.NoError(t, err)
		assert.NoError(t, proposal.Valid())
		client.handleChannelProposal(peer, proposal)
	})

	t.Run("connection-closed-before-accepting-proposal", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		callback := func(proposal *ChannelProposalReq, responder *ProposalResponder) {
			assert.NoError(t, proposal.Valid())
			assert.NoError(t, responder.peer.Close())

			id := wallettest.NewRandomAccount(rng)
			channel, err := responder.Accept(ctx, ProposalAcc{id})
			assert.Nil(t, channel)
			assert.Error(t, err)
		}
		proposalHandler := NewSimpleHandler(callback)
		client := newClient(rng, proposalHandler)

		peer, err := client.peers.Get(ctx, server.id.Address())
		assert.NotNil(t, peer)
		assert.NoError(t, err)

		proposal := newProposal(rng, client.id.Address())
		client.handleChannelProposal(peer, proposal)
	})

	t.Run("context-done-before-accepting-proposal", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		callback := func(proposal *ChannelProposalReq, responder *ProposalResponder) {
			assert.NoError(t, proposal.Valid())

			id := wallettest.NewRandomAccount(rng)
			cancel()
			channel, err := responder.Accept(ctx, ProposalAcc{id})
			assert.Nil(t, channel)
			assert.Error(t, err)
		}
		proposalHandler := NewSimpleHandler(callback)
		client := newClient(rng, proposalHandler)

		peer, err := client.peers.Get(ctx, server.id.Address())
		assert.NotNil(t, peer)
		assert.NoError(t, err)

		proposal := newProposal(rng, client.id.Address())
		client.handleChannelProposal(peer, proposal)
	})

}

func TestClient_validTwoPartyProposal(t *testing.T) {
	rng := rand.New(rand.NewSource(0xdeadbeef))

	// dummy client that only has an id
	c := &Client{
		id: wallettest.NewRandomAccount(rng),
	}
	validProp := *newRandomValidChannelProposalReq(rng, 2)
	validProp.PeerAddrs[0] = c.id.Address() // set us as the proposer
	peerAddr := validProp.PeerAddrs[1]      // peer at 1 as receiver
	require.False(t, peerAddr.Equals(c.id.Address()))
	require.Len(t, validProp.PeerAddrs, 2)

	validProp3Peers := *newRandomValidChannelProposalReq(rng, 3)
	invalidProp := validProp          // shallow copy
	invalidProp.ChallengeDuration = 0 // invalidate

	tests := []struct {
		prop     *ChannelProposalReq
		ourIdx   int
		peerAddr wallet.Address
		valid    bool
	}{
		{
			&validProp,
			0, peerAddr, true,
		},
		// test all three invalid combinations of peer address, index
		{
			&validProp,
			1, peerAddr, false, // wrong ourIdx
		},
		{
			&validProp,
			0, c.id.Address(), false, // wrong peerAddr (ours)
		},
		{
			&validProp,
			1, c.id.Address(), false, // wrong index, wrong peer address
		},
		{
			&validProp3Peers, // valid proposal but three peers
			0, peerAddr, false,
		},
		{
			&invalidProp, // invalid proposal, correct other params
			0, peerAddr, false,
		},
	}

	for i, tt := range tests {
		valid := c.validTwoPartyProposal(tt.prop, tt.ourIdx, tt.peerAddr)
		if tt.valid && valid != nil {
			t.Errorf("[%d] Exptected proposal to be valid but got: %v", i, valid)
		} else if !tt.valid && valid == nil {
			t.Errorf("[%d] Exptected proposal to be invalid", i)
		}
	}
}

func newRandomValidChannelProposalReq(rng *rand.Rand, numPeers int) *ChannelProposalReq {
	peerAddrs := make([]peer.Address, numPeers)
	for i := 0; i < numPeers; i++ {
		peerAddrs[i] = wallettest.NewRandomAddress(rng)
	}
	data := channeltest.NewRandomData(rng)
	alloc := channeltest.NewRandomAllocation(rng, numPeers)
	alloc.Locked = nil // make valid InitBals
	participantAddr := wallettest.NewRandomAddress(rng)
	return &ChannelProposalReq{
		ChallengeDuration: rng.Uint64(),
		Nonce:             big.NewInt(rng.Int63()),
		ParticipantAddr:   participantAddr,
		AppDef:            channeltest.NewRandomApp(rng).Def(),
		InitData:          data,
		InitBals:          alloc,
		PeerAddrs:         peerAddrs,
	}
}
