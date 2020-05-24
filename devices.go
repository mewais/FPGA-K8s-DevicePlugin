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
	pluginapi "github.com/mewais/FPGA-K8s-DevicePlugin/v1beta1"
	log "github.com/sirupsen/logrus"
)

const (
	FREE      int = 0
	USED      int = 1
	BLOCKED   int = 2
	UNHEALTHY int = 3
)

// FIXME: This var is all hypothetical. Change the numbers later
var tenants = map[string]map[string]int{
	// ALVEO board can hold 6 tenants with the Galapagos shell
	"alveo": map[string]int{
		"tenant": 6,
	},
	// SideWinder 100 board can hold 6 tenants with the Galapagos Shell
	"sidewinder-100": map[string]int{
		"tenant": 6,
	},
	// Hypothetical board can hold 2 big tenants, and 4 small ones with the Galapagos Shell
	// This is an example of non uniform PR division
	"HYPOTHETICAL": map[string]int{
		"big-tenant":   2,
		"small-tenant": 2,
	},
}

type FPGADevice struct {
	pluginapi.Device
	// the status of an FPGA device
	// 0 for free
	// 1 for used
	// 2 for blocked because of subdevice being used
	// 3 for being unhealthy
	status   int
	children []*FPGATenantDevice
}

type FPGATenantDevice struct {
	pluginapi.Device
	// the status of an FPGA tenant device
	// 0 for free
	// 1 for used
	// 2 for blocked because of parent being used
	// 3 for being unhealthy
	status int
	parent *FPGADevice
}

func (device *FPGADevice) SetFree() {
	device.status = FREE
	device.Health = pluginapi.Healthy
	log.WithFields(log.Fields{
		"ID": device.ID,
	}).Info("FPGA device is now free")
	// Also set our children to free
	for _, child := range device.children {
		child.status = FREE
		child.Health = pluginapi.Healthy
		log.WithFields(log.Fields{
			"ID": child.ID,
		}).Info("FPGA tenant device is now free")
	}
}

func (device *FPGATenantDevice) SetFree() {
	if device.parent.status == USED {
		log.Fatal("Impossible case, shouldn't be able to change our status when our parent is being used")
		return
	}
	device.status = FREE
	device.Health = pluginapi.Healthy
	log.WithFields(log.Fields{
		"ID": device.ID,
	}).Info("FPGA tenant device is now free")
	// If all other children of our parent are free
	// set the parent free too
	free := true
	health := pluginapi.Healthy
	for _, child := range device.parent.children {
		if child == device {
			continue
		}
		if child.status != FREE {
			free = false
		}
		if child.Health != pluginapi.Healthy {
			health = pluginapi.Unhealthy
		}
	}
	if free {
		device.parent.status = FREE
		device.parent.Health = health
		log.WithFields(log.Fields{
			"ID": device.parent.ID,
		}).Info("FPGA device is now free")
	}
}

func (device *FPGADevice) SetUsed() {
	device.status = USED
	device.Health = pluginapi.Healthy
	log.WithFields(log.Fields{
		"ID": device.ID,
	}).Info("FPGA device is now used")
	// set children to blocked
	for _, child := range device.children {
		child.status = BLOCKED
		child.Health = pluginapi.Healthy
		log.WithFields(log.Fields{
			"ID": child.ID,
		}).Info("FPGA tenant device is now blocked")
	}
}

func (device *FPGATenantDevice) SetUsed() {
	if device.parent.status == USED {
		log.Fatal("Impossible case, shouldn't be able to change our status when our parent is being used")
		return
	}
	device.status = USED
	device.Health = pluginapi.Healthy
	log.WithFields(log.Fields{
		"ID": device.ID,
	}).Info("FPGA tenant device is now used")
	if device.parent.status != BLOCKED {
		device.parent.status = BLOCKED
		device.parent.Health = pluginapi.Healthy
		log.WithFields(log.Fields{
			"ID": device.parent.ID,
		}).Info("FPGA device is now blocked")
	}
}

func (device *FPGADevice) SetUnhealthy() {
	device.status = UNHEALTHY
	device.Health = pluginapi.Unhealthy
	for _, child := range device.children {
		child.status = UNHEALTHY
		child.Health = pluginapi.Unhealthy
	}
}

func (device *FPGATenantDevice) SetUnhealthy() {
	device.status = UNHEALTHY
	device.Health = pluginapi.Unhealthy
	device.parent.status = UNHEALTHY
	device.parent.Health = pluginapi.Unhealthy
}

func (device *FPGADevice) Reset() error {
	// TODO: Reset the FPGA by installing an empty bitstream
	return nil
}

func (device *FPGATenantDevice) Reset() error {
	// TODO: Reset the FPGA PR area by installing an empty bitstream
	return nil
}
