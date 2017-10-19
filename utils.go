package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/rancher/rancher-host-local-ipam/allocator"
	"golang.org/x/net/context"
)

func getRequestedIPByLabel(cid string) (string, error) {
	dClient, err := client.NewEnvClient()
	if err != nil {
		return "", err
	}

	container, err := dClient.ContainerInspect(context.Background(), cid)
	if err != nil {
		return "", err
	}

	requestedIP, ok := container.Config.Labels["io.rancher.container.requested_ip"]
	if ok {
		return requestedIP, nil
	}

	return "", nil
}

func getAllContainers() (containers []types.Container, err error) {
	dClient, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	options := types.ContainerListOptions{All: true}
	return dClient.ContainerList(context.Background(), options)
}

func cleanHistory(ac *allocator.IPAllocator) error {
	containers, err := getAllContainers()
	if err != nil {
		logrus.Errorf("rancher-host-local-ipam: error getting get all containers: %v", err)
		return err
	}
	currentContainers := map[string]bool{}
	for _, c := range containers {
		currentContainers[c.ID] = true
	}
	logrus.Debugf("rancher-host-local-ipam: currentContainers=%v", currentContainers)

	persistedContainers, err := ac.GetAllContainers()
	if err != nil {
		logrus.Errorf("rancher-host-local-ipam: error gettings persisted containers from allocator: %v", err)
		return err
	}
	logrus.Debugf("rancher-host-local-ipam: persistedContainers=%v", persistedContainers)

	for _, id := range persistedContainers {
		if ok, _ := currentContainers[id]; !ok {
			logrus.Debugf("rancher-host-local-ipam: releasing IP for container=%s", id)
			err = ac.Release(id)
			if err != nil {
				logrus.Errorf("rancher-host-local-ipam: error releasing IP for container=%v: %v", id, err)
				return err
			}
		}
	}

	return nil
}
