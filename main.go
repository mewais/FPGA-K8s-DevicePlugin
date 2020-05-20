// Copyright (C) 2020 Mohammad Ewais
// This file is part of FPGA-K8s-DevicePlugin <https://github.com/mewais/FPGA-K8s-DevicePlugin>.
//
// FPGA-k8s-DevicePlugin is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// dogtag is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with dogtag.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"flag"
	"os"
	"syscall"

	pluginapi "github.com/mewais/FPGA-K8s-DevicePlugin/v1beta1"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Parse arguments
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	logLevel := flag.String("log-level", "info", "Define the logging level: error, info, debug.")
	help := flag.Bool("help", false, "Print this help message.")
	flag.Parse()

	// Get log level
	switch *logLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	default:
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Print the help message
	if *help {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Start the filesystem watcher. This gets notified everytime
	// a path is modified. TODO: Explain what this does
	log.Info("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		log.WithFields(log.Fields{
			"Error": err,
		}).Error("Failed to created FS watcher.")
		os.Exit(1)
	}
	defer watcher.Close()

	// Start the OS watcher, this is basically a signal handler
	log.Info("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Get all the devices
	log.Info("Getting Devices.")
	plugins, tenantPlugins := getAllDevices()

	// Start all
}
