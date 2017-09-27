// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/rancher/rancher-host-local-ipam/allocator"
	"github.com/rancher/rancher-host-local-ipam/backend/disk"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
)

const (
	metadataURLTemplate    = "http://%s/2016-07-29"
	defaultMetadataAddress = "169.254.169.250"
)

func main() {
	skel.PluginMain(cmdAdd, cmdDel, version.Legacy)
}

func cmdAdd(args *skel.CmdArgs) error {
	ipamConf, err := allocator.LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}

	if ipamConf.IsDebugLevel == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if ipamConf.LogToFile != "" {
		f, err := os.OpenFile(ipamConf.LogToFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err == nil && f != nil {
			logrus.SetOutput(f)
			defer f.Close()
		}
	}

	store, err := disk.New(ipamConf.Name)
	if err != nil {
		return err
	}
	defer store.Close()

	ac, err := allocator.NewIPAllocator(ipamConf, store)
	if err != nil {
		return err
	}

	ipConf, err := ac.Get(args.ContainerID)
	if err != nil {
		return err
	}

	r := &types.Result{
		IP4: ipConf,
	}
	return r.Print()
}

func cmdDel(args *skel.CmdArgs) error {
	ipamConf, err := allocator.LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}

	if ipamConf.IsDebugLevel == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if ipamConf.LogToFile != "" {
		f, err := os.OpenFile(ipamConf.LogToFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err == nil && f != nil {
			logrus.SetOutput(f)
			defer f.Close()
		}
	}

	metadataAddress := os.Getenv("RANCHER_METADATA_ADDRESS")
	if metadataAddress == "" {
		metadataAddress = defaultMetadataAddress
	}
	metadataURL := fmt.Sprintf(metadataURLTemplate, metadataAddress)
	m, err := metadata.NewClientAndWait(metadataURL)
	if err != nil {
		return err
	}

	containers, err := m.GetContainers()
	if err != nil {
		return err
	}
	selfHost, err := m.GetSelfHost()
	if err != nil {
		return err
	}

	currentContainers := map[string]bool{}
	for _, c := range containers {
		if c.HostUUID == selfHost.UUID {
			currentContainers[c.ExternalId] = true
		}
	}

	store, err := disk.New(ipamConf.Name)
	if err != nil {
		return err
	}
	defer store.Close()

	allocator, err := allocator.NewIPAllocator(ipamConf, store)
	if err != nil {
		return err
	}

	persistContainers, err := allocator.GetAllContainers()
	if err != nil {
		return err
	}

	for _, id := range persistContainers {
		if ok, _ := currentContainers[id]; !ok {
			logrus.Debugf("Release container %s", id)
			err = allocator.Release(id)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
