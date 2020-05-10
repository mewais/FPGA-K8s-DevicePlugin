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

// FIXME: This file is all hypothetical. Change the numbers later

package main

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
