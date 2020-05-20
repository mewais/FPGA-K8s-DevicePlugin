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
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	pluginapi "github.com/mewais/FPGA-K8s-DevicePlugin/v1beta1"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
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
	// Number of devices
	deviceCount int
	// Pointers to the child tenant device plugins
	childPlugins []*FPGATenantDevicePlugin
	// Mutex
	mutex sync.RWMutex
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
	// Number of devices
	deviceCount int
	// Pointer to the parent device plugin
	parentPlugin *FPGADevicePlugin
}

func (plugin *FPGADevicePlugin) fullName() string {
	return join_strings(plugin.vendorName, "/", plugin.boardName)
}

func (plugin *FPGATenantDevicePlugin) fullName() string {
	return join_strings(plugin.vendorName, "/", plugin.boardName, "-", plugin.tenantName)
}

func (plugin *FPGADevicePlugin) socketName() string {
	return join_strings(pluginapi.DevicePluginPath, plugin.boardName, ".sock")
}

func (plugin *FPGATenantDevicePlugin) socketName() string {
	return join_strings(pluginapi.DevicePluginPath, plugin.boardName, "-", plugin.tenantName, ".sock")
}

// FPGA Plugin Constructor, this should take its inputs from system files.
func NewFPGADevicePlugin(vendorName string, boardName string) *FPGADevicePlugin {
	ret := FPGADevicePlugin{
		vendorName:   vendorName,
		boardName:    boardName,
		server:       nil,
		devices:      []*FPGADevice{},
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
	newFPGADevice.ID = join_strings(parentPlugin.fullName(), strconv.Itoa(parentPlugin.deviceCount))
	newFPGADevice.Health = pluginapi.Healthy
	newFPGADevice.status = FREE
	// Add it to plugin
	parentPlugin.devices = append(parentPlugin.devices, newFPGADevice)
	parentPlugin.deviceCount++
	// Create FPGA tenant devices
	for _, childPlugin := range parentPlugin.childPlugins {
		for i := 0; i < tenants[childPlugin.boardName][childPlugin.tenantName]; i++ {
			// Create FPGA tenant device
			newTenantDevice := &FPGATenantDevice{}
			newTenantDevice.ID = join_strings(newFPGADevice.ID, strconv.Itoa(childPlugin.deviceCount))
			newTenantDevice.Health = pluginapi.Healthy
			newTenantDevice.status = FREE
			newTenantDevice.parent = newFPGADevice
			newFPGADevice.children = append(newFPGADevice.children, newTenantDevice)
			// Add it to plugin
			childPlugin.devices = append(childPlugin.devices, newTenantDevice)
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

func (plugin *FPGADevicePlugin) deviceExists(id string) (bool, int) {
	for index, device := range plugin.devices {
		if device.ID == id {
			return true, index
		}
	}
	return false, -1
}

func (plugin *FPGATenantDevicePlugin) deviceExists(id string) (bool, int) {
	for index, device := range plugin.devices {
		if device.ID == id {
			return true, index
		}
	}
	return false, -1
}

func (plugin *FPGADevicePlugin) Start() error {
	plugin.mutex.Lock()

	// Create the server
	plugin.server = grpc.NewServer([]grpc.ServerOption{}...)
	// Register the server
	sock, err := net.Listen("unix", plugin.socketName())
	if err != nil {
		plugin.server = nil
		log.WithFields(log.Fields{
			"Socket": plugin.socketName(),
			"Error":  err,
		}).Error("Cannot listen on socket.")
		return err
	}
	pluginapi.RegisterDevicePluginServer(plugin.server, plugin)
	// Start the server and make sure no errors
	go func() {
		lastCrashTime := time.Now()
		restartCount := 0
		for {
			log.WithFields(log.Fields{
				"Resource": plugin.fullName(),
			}).Info("Starting GRPC server")
			err := plugin.server.Serve(sock)
			if err == nil {
				break
			}
			log.WithFields(log.Fields{
				"Resource": plugin.fullName(),
				"Error":    err,
			}).Error("GRPC server crashed")

			// restart if it has not been too often
			// i.e. if server has crashed more than 5 times and it didn't last more than one hour each time
			if restartCount > 5 {
				log.WithFields(log.Fields{
					"Resource": plugin.fullName(),
					"Error":    err,
				}).Error("GRPC server has repeatedly crashed recently. Quitting")
				break
			}
			timeSinceLastCrash := time.Since(lastCrashTime).Seconds()
			lastCrashTime = time.Now()
			if timeSinceLastCrash > 3600 {
				// it has been one hour since the last crash.. reset the count
				// to reflect on the frequency
				restartCount = 1
			} else {
				restartCount += 1
			}
		}
	}()

	// Wait for server to start by launching a blocking connexion
	conn, err := dial(plugin.socketName(), 5*time.Second)
	if err != nil {
		plugin.server = nil
		log.WithFields(log.Fields{
			"Socket": plugin.socketName(),
			"Error":  err,
		}).Error("Cannot dial socket.")
		return err
	}
	conn.Close()

	// Register our server with kubelet
	conn, err = dial(pluginapi.KubeletSocket, 5*time.Second)
	if err != nil {
		plugin.server = nil
		log.WithFields(log.Fields{
			"Socket": plugin.socketName(),
			"Error":  err,
		}).Error("Cannot dial socket.")
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(plugin.socketName()),
		ResourceName: plugin.fullName(),
		Options: &pluginapi.DevicePluginOptions{
			PreStartRequired:   true,
			PostStopRequired:   true,
			DeallocateRequired: true,
		},
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		plugin.server = nil
		log.WithFields(log.Fields{
			"Error": err,
		}).Error("Cannot register client.")
		return err
	}

	log.WithFields(log.Fields{
		"Resource": plugin.fullName(),
	}).Info("Successfully registered device plugin")

	// Register children device plugins now
	for _, childPlugin := range plugin.childPlugins {
		childPlugin.Start()
	}
	plugin.mutex.Unlock()

	// TODO: start the fixer thread.
	return nil
}

func (plugin *FPGATenantDevicePlugin) Start() error {
	// Create the server
	plugin.server = grpc.NewServer([]grpc.ServerOption{}...)
	// Register the server
	sock, err := net.Listen("unix", plugin.socketName())
	if err != nil {
		plugin.server = nil
		log.WithFields(log.Fields{
			"Socket": plugin.socketName(),
			"Error":  err,
		}).Error("Cannot listen on socket.")
		return err
	}
	pluginapi.RegisterDevicePluginServer(plugin.server, plugin)
	// Start the server and make sure no errors
	go func() {
		lastCrashTime := time.Now()
		restartCount := 0
		for {
			log.WithFields(log.Fields{
				"Resource": plugin.fullName(),
			}).Info("Starting GRPC server")
			err := plugin.server.Serve(sock)
			if err == nil {
				break
			}
			log.WithFields(log.Fields{
				"Resource": plugin.fullName(),
				"Error":    err,
			}).Error("GRPC server crashed")

			// restart if it has not been too often
			// i.e. if server has crashed more than 5 times and it didn't last more than one hour each time
			if restartCount > 5 {
				log.WithFields(log.Fields{
					"Resource": plugin.fullName(),
					"Error":    err,
				}).Error("GRPC server has repeatedly crashed recently. Quitting")
				break
			}
			timeSinceLastCrash := time.Since(lastCrashTime).Seconds()
			lastCrashTime = time.Now()
			if timeSinceLastCrash > 3600 {
				// it has been one hour since the last crash.. reset the count
				// to reflect on the frequency
				restartCount = 1
			} else {
				restartCount += 1
			}
		}
	}()

	// Wait for server to start by launching a blocking connexion
	conn, err := dial(plugin.socketName(), 5*time.Second)
	if err != nil {
		plugin.server = nil
		log.WithFields(log.Fields{
			"Socket": plugin.socketName(),
			"Error":  err,
		}).Error("Cannot dial socket.")
		return err
	}
	conn.Close()

	// Register our server with kubelet
	conn, err = dial(pluginapi.KubeletSocket, 5*time.Second)
	if err != nil {
		plugin.server = nil
		log.WithFields(log.Fields{
			"Socket": plugin.socketName(),
			"Error":  err,
		}).Error("Cannot dial socket.")
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(plugin.socketName()),
		ResourceName: plugin.fullName(),
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		plugin.server = nil
		log.WithFields(log.Fields{
			"Error": err,
		}).Error("Cannot register client.")
		return err
	}

	log.WithFields(log.Fields{
		"Resource": plugin.fullName(),
	}).Info("Successfully registered device plugin")

	return nil
}

// Stop the gRPC server.
func (plugin *FPGADevicePlugin) Stop() error {
	// Lock the mutex
	plugin.mutex.Lock()

	var err error
	err = nil
	if plugin == nil {
		log.Fatal("Attempting to stop a non existing plugin server")
		err = errors.New("Attempting to stop a non existing plugin server")
		return err
	}
	if plugin.server == nil {
		return nil
	}
	log.WithFields(log.Fields{
		"Resource": plugin.fullName(),
		"Socket":   plugin.socketName(),
	}).Info("Stopping plugin server.")
	// Stop the server
	plugin.server.Stop()
	plugin.server = nil
	// Remove the socket
	if err = os.Remove(plugin.socketName()); err != nil && !os.IsNotExist(err) {
		log.WithFields(log.Fields{
			"Socket": plugin.socketName(),
			"Error":  err,
		}).Error("Failed to remove socket")
	} else {
		err = nil
	}
	// Stop tenant plugin servers
	for _, childPlugin := range plugin.childPlugins {
		childPlugin.Stop()
	}
	// Stop all FPGAs and reset their status
	for _, device := range plugin.devices {
		if device.status == USED || device.status == BLOCKED {
			err = device.Reset()
			if err != nil {
				device.SetUnhealthy()
				log.WithFields(log.Fields{
					"ID":    device.ID,
					"Error": err,
				}).Error("Failed to clear FPGA device. Device is now unhealthy")
			} else {
				device.SetFree()
			}
		}
		// UNHEALTHY devices remain unhealthy
		// FREE devices require no action
	}

	// Unlock the mutex
	plugin.mutex.Unlock()
	return err
}

// Stop the gRPC server.
func (plugin *FPGATenantDevicePlugin) Stop() error {
	var err error
	err = nil
	if plugin == nil {
		log.Fatal("Attempting to stop a non existing plugin server")
		err = errors.New("Attempting to stop a non existing plugin server")
		return err
	}
	if plugin.server == nil {
		return nil
	}
	log.WithFields(log.Fields{
		"Resource": plugin.fullName(),
		"Socket":   plugin.socketName(),
	}).Info("Stopping plugin server.")
	// Stop the server
	plugin.server.Stop()
	plugin.server = nil
	// Remove the socket
	if err = os.Remove(plugin.socketName()); err != nil && !os.IsNotExist(err) {
		log.WithFields(log.Fields{
			"Socket": plugin.socketName(),
			"Error":  err,
		}).Error("Failed to remove socket")
	} else {
		err = nil
	}
	// We don't need to stop PR tenants, parent already wiped FPGAs clean
	return err
}

// dial establishes the gRPC communication with the registered device plugin.
func dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}

// Allocate entire FPGAs, disabling partial FPGA tenancy in the process.
func (plugin *FPGADevicePlugin) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	plugin.mutex.Lock()
	responses := pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		log.WithFields(log.Fields{
			"Resource": plugin.fullName(),
			"IDs":      req.DevicesIDs,
		}).Info("FPGAs requested for allocation")
		for _, id := range req.DevicesIDs {
			exists, index := plugin.deviceExists(id)
			if !exists {
				log.WithFields(log.Fields{
					"Resource": plugin.fullName(),
					"ID":       id,
				}).Error("Invalid allocation request. Resource doesn't exist")
				plugin.mutex.Unlock()
				return nil, fmt.Errorf("invalid allocation request for unavailable resource '%s': unknown device: %s", plugin.fullName(), id)
			}
			if plugin.devices[index].status != FREE {
				log.WithFields(log.Fields{
					"Resource": plugin.fullName(),
					"ID":       id,
					"Status":   plugin.devices[index].status,
				}).Error("Invalid allocation request. Resource is busy")
				plugin.mutex.Unlock()
				return nil, fmt.Errorf("invalid allocation request for busy resource '%s': unknown device: %s", plugin.fullName(), id)
			}
		}

		response := pluginapi.ContainerAllocateResponse{
			// TODO: Fill this up
		}

		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}

	// If we are here, it means the request didn't have any errors, we can start killing off the FPGA tenants to
	// be able to serve FPGAs.
	for _, req := range reqs.ContainerRequests {
		for _, id := range req.DevicesIDs {
			exists, index := plugin.deviceExists(id)
			if !exists {
				log.WithFields(log.Fields{
					"Resource": plugin.fullName(),
					"ID":       id,
				}).Fatal("Possible race condition, device should exist")
				plugin.mutex.Unlock()
				os.Exit(2)
			}
			plugin.devices[index].SetUsed()
		}
	}

	plugin.mutex.Unlock()
	return &responses, nil
}

// Allocate FPGA tenants, disabling entire FPGA allocation in the process.
func (plugin *FPGATenantDevicePlugin) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	plugin.parentPlugin.mutex.Lock()
	responses := pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		log.WithFields(log.Fields{
			"Resource": plugin.fullName(),
			"IDs":      req.DevicesIDs,
		}).Info("FPGAs requested for allocation")
		for _, id := range req.DevicesIDs {
			exists, index := plugin.deviceExists(id)
			if !exists {
				log.WithFields(log.Fields{
					"Resource": plugin.fullName(),
					"ID":       id,
				}).Error("Invalid allocation request. Resource doesn't exist")
				plugin.parentPlugin.mutex.Unlock()
				return nil, fmt.Errorf("invalid allocation request for unavailable resource '%s': unknown device: %s", plugin.fullName(), id)
			}
			if plugin.devices[index].status != FREE {
				log.WithFields(log.Fields{
					"Resource": plugin.fullName(),
					"ID":       id,
					"Status":   plugin.devices[index].status,
				}).Error("Invalid allocation request. Resource is busy")
				plugin.parentPlugin.mutex.Unlock()
				return nil, fmt.Errorf("invalid allocation request for busy resource '%s': unknown device: %s", plugin.fullName(), id)
			}
		}

		response := pluginapi.ContainerAllocateResponse{
			// TODO: Fill this up
		}

		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}

	// If we are here, it means the request didn't have any errors, we can start killing off the FPGA tenants to
	// be able to serve FPGAs.
	for _, req := range reqs.ContainerRequests {
		for _, id := range req.DevicesIDs {
			exists, index := plugin.deviceExists(id)
			if !exists {
				log.WithFields(log.Fields{
					"Resource": plugin.fullName(),
					"ID":       id,
				}).Fatal("Possible race condition, device should exist")
				plugin.parentPlugin.mutex.Unlock()
				os.Exit(2)
			}
			plugin.devices[index].SetUsed()
		}
	}

	plugin.parentPlugin.mutex.Unlock()
	return &responses, nil
}
