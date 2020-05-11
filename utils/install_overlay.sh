#!/bin/bash

# Copyright (C) 2020 Mohammad Ewais
# This file is part of FPGA-K8s-DevicePlugin <https://github.com/mewais/FPGA-K8s-DevicePlugin>.
#
# FPGA-k8s-DevicePlugin is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# dogtag is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with dogtag.  If not, see <http://www.gnu.org/licenses/>.

if [ -z "$1" ]; then
    echo "No overlay file provided"
    exit 1
fi

echo "Compiling overlay"
dtc -O dtb -o $1.dtbo -b 0 -@ $1

echo "Installing overlay"
mkdir /configfs
if [ ! -e /configfs/device-tree ]; then
    mount -t configfs configfs /configfs;
    mkdir /configfs/device-tree/overlays/full;
    echo $1.dtbo > /configfs/device-tree/overlays/full/path;
else
    if [ ! -e /configfs/device-tree/overlays/full ]; then
        mkdir /configfs/device-tree/overlays/full;
        echo $1.dtbo > /configfs/device-tree/overlays/full/path;
    else
        pci_remove;
        rmdir /configfs/device-tree/overlays/full;
        mkdir /configfs/device-tree/overlays/full;
        echo $1.dtbo > /configfs/device-tree/overlays/full/path;
    fi;
fi
