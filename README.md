# FPGA device plugins for K8s

This repo contains a device plugin for xilinx FPGAs. It has the following features and differences from the [official Xilinx device](https://github.com/Xilinx/FPGA_as_a_Service/tree/master/k8s-fpga-device-plugin):
- The device plugin allows you to discover FPGAs connected to K8s nodes. Unlike the official xilinx plugin, one deployment of this device plugin can find multiple types of FPGAs and report them as separate devices. For example: rather than advertising `xilinx.com/fpga`, it can advertise `xilinx.com/alveo` and/or `fidus.com/sidewinder-100`. This makes it suitable for deployment in clusters with heterogeneous resources.
- The device plugin allows for using the FPGAs inside deployed K8s containers in two ways:
  - Whole FPGAs: allows a container to request entire FPGAs of specific types as needed. No SDAccel or other shells required.
  - Partial FPGAs: which utilizes the [Galapagos](https://github.com/UofT-HPRC/galapagos) framework to allow multiple tenants/containers to utilize parts of the same FPGA.
- This device plugin utilizes a [modified device plugin API](https://github.com/kubernetes/kubernetes/pull/91190) which allows for deallocating FPGA resources as needed, otherwise there's a potential for FPGAs to waste power and pollute networks after containers are finished running.

## Work In Progress
This is a heavy work in progress, it is still under construction

## How to use
- You must the modified kubelet for the plugin to work
  - Install kubernetes version 1.18.2 on your nodes. Currently this is the only version where our modifications have been applied.
  - clone the modified [kubernetes repo](https://github.com/mewais/kubernetes.git)
  - `cd kubernetes`
  - `git checkout v1.18.2-FPGA`
  - `make release`
  - The output will be in `_output/release-tars/kubernetes-node-linux-ARCH.tar.gz`
  - When uncompressed, you will find the `kubelet` plugin in `/kubernetes/node/bin/`
  - Copy the binary to the nodes, specifically to the path `/usr/bin/kubelet`
  - You're good to go
- MPSoC nodes must install the corresponding device tree overlays available in `utils/`.
- Nodes with PCIe connected FPGAs are still not supported.
- Deploy using the `fpga-device-plugin.yaml`
