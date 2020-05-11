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
	"io/ioutil"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// The hierarchy of this is a bit weird. We have `Device Plugins`, each plugin
// can only advertise one type of devices, but as many devices as we have of that
// type. Exmaple: Plugin1 can only advertise sidewinders, if we have a 100 sidewinders
// connected it will be able to advertise all of them, but cannot advertise alveos
// For each resource type, we need a different plugin.
//
// We have two types of resources, and those are as follows:
// 1. Entire FPGAs, can be used by one app that occupies it in its entirety
// 2. One FPGA tenant, through the use of an FPGA Shell to handle resource
//	allocation and multi tenant communications
// We offer both as devices, for example: We offer an FPGA board, and 6 tenants
// (even though they're the same hardware). If a deployment asks for the entire
// FPGA, we also hide the 6 tenant resources, and if a deployment asks for a
// single FPGA tenant, we hide the entire FPGA resource.

type FPGADevicePlugin struct {
	// These two strings are what we use to identify our FPGAs,
	// for example:
	//		fidus.com/sidewinder-100
	//		xilinx.com/alveo
	vendorName string
	boardName  string
	// The server for this plugin
	server *grpc.Server
	// The list of IDs of FPGA devices of this type in the system
	devices []*FPGADevice
	// the status of every FPGA device of this type in the system
	// 0 for free
	// 1 for used
	// 2 for blocked because of subdevice being used
	status []int
	// Number of devices
	deviceCount int
	// Pointers to the child tenant device plugins
	childPlugins []*FPGATenantDevicePlugin
}

type FPGATenantDevicePlugin struct {
	// These three strings are what we use to identify our FPGA Tenant areas,
	// the type is useful for heterogeneous tenancy division of FPGAs.
	// for example:
	//		fidus.com/sidewinder-100-size1
	//		xilinx.com/alveo-uniform
	vendorName string
	boardName  string
	tenantName string
	// The server for this plugin
	server *grpc.Server
	// The id of this tenant in its parent FPGA device.
	devices []*FPGATenantDevice
	// the status of every FPGA tenant of this type in the system
	// 0 for free
	// 1 for used
	// 2 for blocked because of parent being used
	status []int
	// Number of devices
	deviceCount int
	// Pointer to the parent device plugin
	parentPlugin *FPGADevicePlugin
}

// FPGA Plugin Constructor, this should take its inputs from system files.
func NewFPGADevicePlugin(vendorName string, boardName string) *FPGADevicePlugin {
	ret := FPGADevicePlugin{
		vendorName:   vendorName,
		boardName:    boardName,
		server:       nil,
		devices:      []*FPGADevice{},
		status:       []int{},
		deviceCount:  0,
		childPlugins: []*FPGATenantDevicePlugin{},
	}
	return &ret
}

// FPGA Tenant Constructor, constructs one if the FPGA is divided uniformly to PR regions,
// and constructs multiples otherwise
func NewFPGATenantDevicePlugins(parentPlugin *FPGADevicePlugin) []*FPGATenantDevicePlugin {
	var ret []*FPGATenantDevicePlugin
	for tenantName, _ := range tenants[parentPlugin.boardName] {
		newTenantPlugin := &FPGATenantDevicePlugin{
			vendorName:   parentPlugin.vendorName,
			boardName:    parentPlugin.boardName,
			tenantName:   tenantName,
			server:       nil,
			devices:      []*FPGATenantDevice{},
			status:       []int{},
			deviceCount:  0,
			parentPlugin: parentPlugin,
		}
		parentPlugin.childPlugins = append(parentPlugin.childPlugins, newTenantPlugin)
		ret = append(ret, newTenantPlugin)
	}
	return ret
}

func addDevice(parentPlugin *FPGADevicePlugin) {
	// Create FPGA device
	newFPGADevice := &FPGADevice{}
	newFPGADevice.ID = strconv.Itoa(parentPlugin.deviceCount)
	newFPGADevice.Health = pluginapi.Healthy
	// Add it to plugin
	parentPlugin.devices = append(parentPlugin.devices, newFPGADevice)
	parentPlugin.status = append(parentPlugin.status, 0)
	parentPlugin.deviceCount++
	// Create FPGA tenant devices
	for _, childPlugin := range parentPlugin.childPlugins {
		for i := 0; i < tenants[childPlugin.boardName][childPlugin.tenantName]; i++ {
			// Create FPGA tenant device
			newTenantDevice := &FPGATenantDevice{}
			newTenantDevice.ID = join_strings(newFPGADevice.ID, strconv.Itoa(childPlugin.deviceCount))
			newTenantDevice.Health = pluginapi.Healthy
			newTenantDevice.parent = newFPGADevice
			newFPGADevice.children = append(newFPGADevice.children, newTenantDevice)
			// Add it to plugin
			childPlugin.devices = append(childPlugin.devices, newTenantDevice)
			childPlugin.status = append(childPlugin.status, 0)
			childPlugin.deviceCount++
		}
	}
}

// This is unused now, but will be useful in the case of multi FPGAs connected through PCIe
// Check if a plugin for this FPGA type has already been created, and return it if found
func havePlugin(vendorName string, boardName string, plugins []*FPGADevicePlugin) int {
	found := -1
	for index, element := range plugins {
		if element.vendorName == vendorName && element.boardName == boardName {
			found = index
			break
		}
	}
	return found
}

// Create all devices, this searches the system for all connected FPGAs
// and constructs all of them
// TODO: Right now, this only works for MPSoCs that have device trees
// overlayes similar to those in `utils/`. Need to add support for PCIe
// connected FPGAs.
func getAllDevices() ([]*FPGADevicePlugin, []*FPGATenantDevicePlugin) {
	var devicePlugins []*FPGADevicePlugin
	var tenantDevicePlugins []*FPGATenantDevicePlugin
	// We expect SoC FPGAs info to be at `/proc/device-tree/fpga-full/`
	// according to the sample device trees in `utils`.
	if _, err := os.Stat("/proc/device-tree/fpga-full"); err == nil {
		// FPGA exists, try getting the vendor and board info
		dat1, err := ioutil.ReadFile("/proc/device-tree/vendor")
		if err != nil {
			log.Warn("Could not read FPGA Info. Did you install the device tree overlay?")
			return devicePlugins, tenantDevicePlugins
		}
		dat2, err := ioutil.ReadFile("/proc/device-tree/board")
		if err != nil {
			log.Warn("Could not read FPGA Info. Did you install the device tree overlay?")
			return devicePlugins, tenantDevicePlugins
		}
		vendorName := string(dat1)
		boardName := string(dat2)
		// Now we can create the device plugin
		// Note that in MPSoCs, there's typically one FPGA, so we don't need
		// to check whether or not a device plugin has been created before.
		newDevicePlugin := NewFPGADevicePlugin(vendorName, boardName)
		devicePlugins = append(devicePlugins, newDevicePlugin)
		log.WithFields(log.Fields{
			"Vendor": vendorName,
			"Board":  boardName,
		}).Info("Found MPSoC FPGA connected.")
		// And the corresponding tenant device plugin
		tenantDevicePlugins = append(tenantDevicePlugins, NewFPGATenantDevicePlugins(newDevicePlugin)...)
		// Now we add the actual devices
		addDevice(newDevicePlugin)
	} else {
		log.Info("No MPSoC FPGA found.")
	}
	// TODO: Check for PCIe connected FPGAs
	return devicePlugins, tenantDevicePlugins
}
