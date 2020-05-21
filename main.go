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

	"github.com/fsnotify/fsnotify"
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
	fsWatcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		log.WithFields(log.Fields{
			"Error": err,
			"Path":  pluginapi.DevicePluginPath,
		}).Error("Failed to create FS watcher.")
		os.Exit(1)
	}
	defer fsWatcher.Close()

	// Start the OS watcher, this is basically a signal handler
	log.Info("Starting OS watcher.")
	sigsWatcher := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Get all the devices
	log.Info("Getting Devices.")
	plugins, _ := getAllDevices()

Lifetime:
	// Start all
	for {
		// Initial reset and start plugins
		for _, plugin := range plugins {
			err := plugin.Stop()
			if err != nil {
				log.WithFields(log.Fields{
					"Error": err,
				}).Debug("Plugin Stopping failed, skipping")
				// Stop will take care of printing the errors
				// just cancel
				continue
			}
			err = plugin.Start()
			if err != nil {
				log.WithFields(log.Fields{
					"Error": err,
				}).Debug("Plugin Starting failed, skipping")
				// Start will take care of printing the errors
				// just cancel
				continue
			}
		}

	PostInit:
		// Remaining lifetime of plugins
		for {
			select {
			// Check for kubelet restart
			case event := <-fsWatcher.Events:
				if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
					log.Info("Kubelet restarted, restarting")
					break PostInit
				}
			// Check for filesystem errors
			case err := <-fsWatcher.Errors:
				log.WithFields(log.Fields{
					"Error": err,
				}).Info("FS Watcher Error")
			// Check for signal interrupts
			case signal := <-sigsWatcher:
				switch signal {
				case syscall.SIGHUP:
					log.Info("Received SIGHUP, restarting.")
					break PostInit
				default:
					log.WithFields(log.Fields{
						"Signal": signal,
					}).Info("Recieved interrupt, shutting down.")
					break Lifetime
				}
			}
		}
	}
}
