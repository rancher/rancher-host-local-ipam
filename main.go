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
		logrus.Errorf("rancher-host-local-ipam: error creating store: %v", err)
		return err
	}
	defer store.Close()

	ac, err := allocator.NewIPAllocator(ipamConf, store)
	if err != nil {
		logrus.Errorf("rancher-host-local-ipam: error creating allocator: %v", err)
		return err
	}

	ipConf, err := ac.Get(args.ContainerID)
	if err != nil {
		logrus.Errorf("rancher-host-local-ipam: error getting IP address from allocator: %v", err)
		return err
	}
	logrus.Debugf("rancher-host-local-ipam: ipConf: %v", ipConf)

	r := &types.Result{
		IP4: ipConf,
	}
	logrus.Infof("rancher-host-local-ipam: for container=%v got result=", args.ContainerID, *r)

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
		logrus.Errorf("rancher-host-local-ipam: error creating metadata client: %v", err)
		return err
	}

	containers, err := m.GetContainers()
	if err != nil {
		logrus.Errorf("rancher-host-local-ipam: error fetching containers from metadata: %v", err)
		return err
	}
	selfHost, err := m.GetSelfHost()
	if err != nil {
		logrus.Errorf("rancher-host-local-ipam: error fetching self from metadata: %v", err)
		return err
	}

	currentContainers := map[string]bool{}
	for _, c := range containers {
		if c.HostUUID == selfHost.UUID {
			currentContainers[c.ExternalId] = true
		}
	}
	logrus.Debugf("rancher-host-local-ipam: currentContainers=%v", currentContainers)

	store, err := disk.New(ipamConf.Name)
	if err != nil {
		logrus.Errorf("rancher-host-local-ipam: error creating store: %v", err)
		return err
	}
	defer store.Close()

	allocator, err := allocator.NewIPAllocator(ipamConf, store)
	if err != nil {
		logrus.Errorf("rancher-host-local-ipam: error creating allocator: %v", err)
		return err
	}

	persistedContainers, err := allocator.GetAllContainers()
	if err != nil {
		logrus.Errorf("rancher-host-local-ipam: error gettings persisted containers from allocator: %v", err)
		return err
	}
	logrus.Debugf("rancher-host-local-ipam: persistedContainers=%v", persistedContainers)

	for _, id := range persistedContainers {
		if ok, _ := currentContainers[id]; !ok {
			logrus.Debugf("rancher-host-local-ipam: releasing IP for container=%s", id)
			err = allocator.Release(id)
			if err != nil {
				logrus.Errorf("rancher-host-local-ipam: error releasing IP for container=%v: %v", id, err)
				return err
			}
		}
	}

	return nil
}
