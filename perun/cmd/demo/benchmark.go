// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package demo

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/montanaflynn/stats"
	"github.com/pkg/errors"

	"perun.network/go-perun/channel"
)

type run struct {
	// data are the recorded times for sendUpdate in nanoseconds
	data []float64
	// start of the last run
	start time.Time
}

func (r *run) Start() {
	r.start = time.Now()
}

func (r *run) Stop() {
	r.data = append(r.data, float64(time.Since(r.start).Nanoseconds()))
}

func (r *run) String() string {
	sum, _ := stats.Sum(r.data)
	min, _ := stats.Min(r.data)
	max, _ := stats.Max(r.data)
	median, _ := stats.Median(r.data)
	stddev, _ := stats.StdDevP(r.data)
	f := (float64(len(r.data)) / sum) * float64(time.Second.Nanoseconds())

	return fmt.Sprintf("N\ttx/s\tSum\tMin\tMax\tMedian\tStddev\t\n%d\t%g\t%v\t%v\t%v\t%v\t%v\t", len(r.data), f, time.Duration(sum)*time.Nanosecond, time.Duration(min)*time.Nanosecond, time.Duration(max)*time.Nanosecond, time.Duration(median)*time.Nanosecond, time.Duration(stddev)*time.Nanosecond)
}

// Benchmark updates the channel with a `peer` `n` times and measures the of every update.
// A statistic is then printed with run.String()
func (n *node) Benchmark(args []string) error {
	peer := n.peers[args[0]]
	x, _ := strconv.Atoi(args[1])
	var r run

	if x < 1 {
		return errors.New("Number of runs cant be less than 1")
	} else if peer == nil {
		return errors.New("Peer not found")
	} else if peer.ch == nil {
		return errors.New("Open a state channel first")
	}

	for i := 0; i < x; i++ {
		r.Start()
		if err := peer.ch.sendUpdate(func(*channel.State) {}, "benchmark"); err != nil {
			return errors.WithMessage(err, "could not send update")
		}
		r.Stop()
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.Debug)
	fmt.Fprintln(w, r.String())
	return w.Flush()
}
