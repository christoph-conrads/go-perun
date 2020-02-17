// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"perun.network/go-perun/log"
	plogrus "perun.network/go-perun/log/logrus"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "perun",
	Short: "Perun Network umbrella executable",
}

var cfgFile string
var rawLvl string

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	rootCmd.PersistentFlags().StringVar(&rawLvl, "log", "warn", "Logrus level")
}

// initConfig reads the config and sets the loglevel.
// The demo configuration will be parsed in the
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config.yaml")
		viper.AddConfigPath(".")
	}
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	lvl, err := logrus.ParseLevel(rawLvl)
	if err != nil {
		log.Fatalf("Unknown loglevel '%s'\n", rawLvl)
	}
	plogrus.Set(lvl, &logrus.TextFormatter{ForceColors: true})
}

// Execute called by rootCmd
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
