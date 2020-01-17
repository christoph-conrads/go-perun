// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package channel

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestPeerTimedOutFundingError(t *testing.T) {
	assert := assert.New(t)
	err := NewAssetFundingError(42, []uint16{1, 2, 3, 4})
	perr, ok := errors.Cause(err).(*AssetFundingError)
	assert.True(ok)
	assert.Equal(42, perr.Asset)
	assert.Equal(Index(1), perr.TimedOutPeers[0])
	assert.Equal(Index(2), perr.TimedOutPeers[1])
	assert.Equal(Index(3), perr.TimedOutPeers[2])
	assert.True(IsAssetFundingError(err))
	assert.True(IsAssetFundingError(perr))
	assert.False(IsAssetFundingError(errors.New("no PeerTimedOutFundingError")))
}
