// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package demo // import "perun.network/go-perun/perun/cmd/demo"

import (
	"time"

	"github.com/spf13/viper"
)

type cfg struct {
	Channel channelConfig
	Node    nodeConfig
	Chain   chainConfig
	Seed    int64
}

type channelConfig struct {
	Timeout              time.Duration
	FundTimeout          time.Duration
	SettleTimeout        time.Duration
	ChallengeDurationSec uint64
}

type nodeConfig struct {
	IP            string
	InPort        uint16
	OutPort       uint16
	DialTimeout   time.Duration
	HandleTimeout time.Duration
}

type chainConfig struct {
	AdjDeployTimeout time.Duration
	AssDeployTimeout time.Duration

	Adjudicator string
	Assetholder string
	// URL the endpoint of your ethereum node / infura eg: ws://10.70.5.70:8546
	URL string
}

var config cfg

// SetConfig called by viper when the config file was parsed
func SetConfig() error {
	if err := viper.Unmarshal(&config); err != nil {
		return err
	}
	return nil
}
