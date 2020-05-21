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
# along with dogtag. If not, see <http://www.gnu.org/licenses/>.

docker-multiarch: docker-amd64 docker-arm64
	rm -rf ~/.docker/manifests/docker.io_uofthprc_fpga-k8s-deviceplugin-latest/
	docker push uofthprc/fpga-k8s-deviceplugin:amd64
	docker push uofthprc/fpga-k8s-deviceplugin:arm64
	docker manifest create uofthprc/fpga-k8s-deviceplugin:latest --amend uofthprc/fpga-k8s-deviceplugin:amd64 --amend uofthprc/fpga-k8s-deviceplugin:arm64
	docker manifest push uofthprc/fpga-k8s-deviceplugin:latest

docker-amd64: Dockerfile.amd64 FPGA-K8s-DevicePlugin-amd64
	docker build -t uofthprc/fpga-k8s-deviceplugin:amd64 -f $< .

docker-arm64: Dockerfile.arm64 FPGA-K8s-DevicePlugin-arm64
	docker build -t uofthprc/fpga-k8s-deviceplugin:arm64 -f $< .

FPGA-K8s-DevicePlugin-amd64: main.go server.go utils.go watcher.go devices.go
	env GOOS=linux GOARCH=amd64 go build -o $@

FPGA-K8s-DevicePlugin-arm64: main.go server.go utils.go watcher.go devices.go
	env GOOS=linux GOARCH=arm64 go build -o $@

clean:
	docker rmi fpga-k8s-device-plugin:amd64
	docker rmi fpga-k8s-device-plugin:arm64
	rm FPGA-K8s-DevicePlugin-* -f
