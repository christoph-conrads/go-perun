// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package demo

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/client"
	"perun.network/go-perun/log"
)

type (
	paymentChannel struct {
		*client.Channel

		log     log.Logger
		handler chan bool
		res     chan handlerRes
	}

	// A handlerRes encapsulates the result of a channel handling request
	handlerRes struct {
		up  client.ChannelUpdate
		err error
	}
)

func newPaymentChannel(ch *client.Channel) *paymentChannel {
	return &paymentChannel{
		Channel: ch,
		log:     log.WithField("channel", ch.ID()),
		handler: make(chan bool, 1),
		res:     make(chan handlerRes),
	}
}
func (ch *paymentChannel) sendMoney(amount *big.Int) error {
	return ch.sendUpdate(
		func(state *channel.State) {
			transferBal(stateBals(state), ch.Idx(), amount)
		}, "sendMoney")
}

func (ch *paymentChannel) sendFinal() error {
	ch.log.Debugf("Sending final state")
	return ch.sendUpdate(func(state *channel.State) {
		state.IsFinal = true
	}, "final")
}

func (ch *paymentChannel) sendUpdate(update func(*channel.State), desc string) error {
	ch.log.Debugf("Sending update: %s", desc)
	ctx, cancel := context.WithTimeout(context.Background(), config.Channel.Timeout)
	defer cancel()

	state := ch.State().Clone()
	update(state)
	state.Version++

	err := ch.Update(ctx, client.ChannelUpdate{
		State:    state,
		ActorIdx: ch.Idx(),
	})
	ch.log.Debugf("Sent update: %s, err: %v", desc, err)
	return err
}

func transferBal(bals []channel.Bal, ourIdx channel.Index, amount *big.Int) {
	a := new(big.Int).Set(amount) // local copy because we mutate it
	otherIdx := ourIdx ^ 1
	ourBal := bals[ourIdx]
	otherBal := bals[otherIdx]
	otherBal.Add(otherBal, a)
	ourBal.Add(ourBal, a.Neg(a))
}

func stateBals(state *channel.State) []channel.Bal {
	return []channel.Bal{state.OfParts[0][0], state.OfParts[1][0]}
}

func (ch *paymentChannel) Handle(update client.ChannelUpdate, res *client.UpdateResponder) {
	if update.State.IsFinal {
		ch.log.Debug("Final payment request")
	} else {
		ch.log.Debug("New payment request")
	}
	ctx, cancel := context.WithTimeout(context.Background(), config.Channel.Timeout)
	defer cancel()
	if err := res.Accept(ctx); err != nil {
		ch.log.Error(errors.WithMessage(err, "handling payment request"))
	}
}

func (ch *paymentChannel) ListenUpdates() {
	ch.log.Trace("Starting update listener")
	ch.Channel.ListenUpdates(ch)
	ch.log.Trace("Stopped update listener")
}

func (ch *paymentChannel) GetBalances() (our, other *big.Int) {
	return ch.State().OfParts[ch.Idx()][0], ch.State().OfParts[1-ch.Idx()][0]
}
