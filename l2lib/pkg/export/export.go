package export

import (
	"fmt"
	"strings"
)

type Mac struct {
	Data string
}

type PCIAddress struct {
	Device, Function string
}

type PTPCaps struct {
	HwRx, HwTx, HwRawClock bool
}

type Iface struct {
	IfName    string
	IfMac     Mac
	IfIndex   int
	IfPci     PCIAddress
	IfPTPCaps PTPCaps
	IfUp      bool
}

type Neighbors struct {
	Local  Iface
	Remote map[string]bool
}

func (mac Mac) String() string {
	return strings.ToUpper(string([]byte(mac.Data)[0:2]) + ":" +
		string([]byte(mac.Data)[2:4]) + ":" +
		string([]byte(mac.Data)[4:6]) + ":" +
		string([]byte(mac.Data)[6:8]) + ":" +
		string([]byte(mac.Data)[8:10]) + ":" +
		string([]byte(mac.Data)[10:12]))
}

// Object representing a ptp interface within a cluster.
type PtpIf struct {
	// Mac address of the Ethernet interface
	MacAddress string
	// Index of the interface in the cluster (node/interface name)
	IfClusterIndex
	// PCI address
	IfPci PCIAddress
}

// Object used to index interfaces in a cluster
type IfClusterIndex struct {
	// interface name
	IfName string
	// node name
	NodeName string
}

func (index IfClusterIndex) String() string {
	return fmt.Sprintf("%s_%s", index.NodeName, index.IfName)
}

func (iface *PtpIf) String() string {
	return fmt.Sprintf("%s : %s", iface.NodeName, iface.IfName)
}

func (iface *PtpIf) String1() string {
	return fmt.Sprintf("index:%s mac:%s", iface.IfClusterIndex, iface.MacAddress)
}
