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
	// The ID of this FPGA in the system
	id int
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
	// The ID of the FPGA in the system
	id int
	// The ID of the tenant in the FPGA
	tenantId int
}

var deviceCount int = 0

// FPGA Constructor, this should take its inputs from system files.
// FIXME: This is only for testing now, so takes names rightaway.
func NewFPGADevicePlugin(vendorName string, boardName string) *FPGADevicePlugin {
	ret := FPGADevicePlugin{
		vendorName: vendorName,
		boardName:  boardName,
		id:         deviceCount,
	}
	deviceCount++
	return &ret
}

// FPGA Tenant Constructor, constructs multiples of them at once
func NewFPGATenantDevicePlugins(devicePlugin *FPGADevicePlugin) []*FPGATenantDevicePlugin {
	var ret []*FPGATenantDevicePlugin
	for tenantName, tenantCount := range tenants[devicePlugin.boardName] {
		for tenantId := 0; tenantId < tenantCount; tenantId++ {
			ret = append(ret, &FPGATenantDevicePlugin{
				vendorName: devicePlugin.vendorName,
				boardName:  devicePlugin.boardName,
				tenantName: tenantName,
				id:         devicePlugin.id,
				tenantId:   tenantCount,
			})
		}
	}
	return ret
}
