package main

import (
	"github.com/openshift/ptp-operator/test/utils/client"
	"github.com/test-network-function/l2discovery/l2lib"
)

func main() {
	client.Client = client.New("")
	l2lib.GlobalL2DiscoveryConfig.SetL2Client(client.Client, client.Client.Config)
	_, _ = l2lib.GlobalL2DiscoveryConfig.GetL2DiscoveryConfig()
}
