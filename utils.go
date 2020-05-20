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

import pluginapi "github.com/mewais/FPGA-K8s-DevicePlugin/v1beta1"

func join_strings(strs ...string) string {
	var ret string
	for _, str := range strs {
		ret += str
	}
	return ret
}

// FIXME: Arrays are not that big for now, but we should improve
// this
func check_array_equality(arr1, arr2 []*pluginapi.Device) bool {
	if len(arr1) != len(arr2) {
		return false
	}
	for _, element1 := range arr1 {
		found := false
		for _, element2 := range arr2 {
			if element1 == element2 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
