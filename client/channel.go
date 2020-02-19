// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package client

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/log"
	"perun.network/go-perun/peer"
	perunsync "perun.network/go-perun/pkg/sync"
	"perun.network/go-perun/wallet"
)

// Channel is the channel controller, progressing the channel state machine and
// executing the channel update and dispute protocols.
//
// Currently, only the two-party protocol is fully implemented.
type Channel struct {
	perunsync.Closer
	log log.Logger

	conn        *channelConn
	machine     channel.StateMachine
	machMtx     sync.RWMutex
	updateSub   chan<- *channel.State
	adjudicator channel.Adjudicator
}

// newChannel is internally used by the Client to create a new channel
// controller after the channel proposal protocol ran successfully.
func newChannel(
	acc wallet.Account,
	peers []*peer.Peer,
	params channel.Params,
	adjudicator channel.Adjudicator,
) (*Channel, error) {
	machine, err := channel.NewStateMachine(acc, params)
	if err != nil {
		return nil, errors.WithMessage(err, "creating state machine")
	}

	// bundle peers into channel connection
	conn, err := newChannelConn(params.ID(), peers, machine.Idx())
	if err != nil {
		return nil, errors.WithMessagef(err, "setting up channel connection")
	}

	logger := log.WithFields(log.Fields{"channel": params.ID(), "id": acc.Address()})
	conn.SetLogger(logger)
	return &Channel{
		log:         logger,
		conn:        conn,
		machine:     *machine,
		adjudicator: adjudicator,
	}, nil
}

// Close closes the channel and all associated peer subscriptions.
func (c *Channel) Close() error {
	if err := c.Closer.Close(); err != nil {
		return err
	}
	return c.conn.Close()
}

func (c *Channel) setLogger(l log.Logger) {
	c.log = l
}

func (c *Channel) logPeer(idx channel.Index) log.Logger {
	return c.log.WithField("peerIdx", idx)
}

// ID returns the channel ID.
func (c *Channel) ID() channel.ID {
	return c.machine.ID()
}

// Idx returns our index in the channel.
func (c *Channel) Idx() channel.Index {
	return c.machine.Idx()
}

// Params returns the channel parameters.
func (c *Channel) Params() *channel.Params {
	return c.machine.Params()
}

// State returns the current state.
// Clone it if you want to modify it.
func (c *Channel) State() *channel.State {
	c.machMtx.RLock()
	defer c.machMtx.RUnlock()

	return c.machine.State()
}

// Phase returns the current phase of the channel state machine.
func (c *Channel) Phase() channel.Phase {
	c.machMtx.RLock()
	defer c.machMtx.RUnlock()

	return c.machine.Phase()
}

// init brings the state machine into the InitSigning phase. It is not callable
// by the user since the Client initializes the channel controller.
// The state machine is not locked as this function is expected to be called
// during the initialization phase of the channel controller.
func (c *Channel) init(initBals *channel.Allocation, initData channel.Data) error {
	return c.machine.Init(*initBals, initData)
}

// initExchangeSigsAndEnable exchanges signatures on the initial state.
// The state machine is not locked as this function is expected to be called
// during the initialization phase of the channel controller.
func (c *Channel) initExchangeSigsAndEnable(ctx context.Context) error {
	sig, err := c.machine.Sig()
	if err != nil {
		return err
	}

	resRecv, err := c.conn.NewUpdateResRecv(0)
	if err != nil {
		return errors.WithMessage(err, "creating update response receiver")
	}
	defer resRecv.Close()

	send := make(chan error)
	go func() {
		send <- c.conn.Send(ctx, &msgChannelUpdateAcc{
			ChannelID: c.ID(),
			Version:   0,
			Sig:       sig,
		})
	}()

	pidx, cm := resRecv.Next(ctx)
	acc, ok := cm.(*msgChannelUpdateAcc)
	if !ok {
		return errors.Errorf(
			"received unexpected message of type (%T) from peer[%d]: %v",
			cm, pidx, cm)
	}

	if err := c.machine.AddSig(pidx, acc.Sig); err != nil {
		return err
	}
	if err := c.machine.EnableInit(); err != nil {
		return err
	}

	return errors.WithMessage(<-send, "sending initial signature")
}

// Settle settles the channel using the Settler. The channel must be in a
// final state.
func (c *Channel) Settle(ctx context.Context) error {
	c.machMtx.Lock()
	defer c.machMtx.Unlock()

	// check final state
	if c.machine.Phase() != channel.Final || !c.machine.State().IsFinal {
		return errors.New("currently, only channels in a final state can be settled")
	}

	if err := c.machine.SetRegistering(); err != nil {
		return err // this should never happen
	}

	req := c.machine.AdjudicatorReq()

	reg, err := c.adjudicator.Register(ctx, req)
	if err != nil {
		return errors.WithMessage(err, "calling Register")
	}
	if err := c.machine.SetRegistered(); err != nil {
		return err // this should never happen
	}

	if reg.Version != req.Tx.Version {
		return errors.Errorf(
			"unexpected version %d registered, expected %d", reg.Version, req.Tx.Version)
	}

	if reg.Timeout.After(time.Now()) {
		c.log.Warnf("Unexpected withdrawal timeout during Settle(). Waiting until %v.", reg.Timeout)
		timeout := time.After(time.Until(reg.Timeout))
		select {
		case <-timeout: // proceed normally with withdrawal
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "ctx done")
		}
	}

	if err := c.machine.SetWithdrawing(); err != nil {
		return err // this should never happen
	}
	if err := c.adjudicator.Withdraw(ctx, req); err != nil {
		return errors.WithMessage(err, "calling Withdraw")
	}
	return c.machine.SetWithdrawn()
}
