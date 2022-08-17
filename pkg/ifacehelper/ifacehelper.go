package ifacehelper

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/test-network-function/l2discovery/l2lib/pkg/export"
	run "github.com/test-network-function/l2discovery/pkg/run"
)

type ipOut struct {
	Ifindex          int           `json:"ifindex"`
	Ifname           string        `json:"ifname"`
	Flags            []string      `json:"flags"`
	Mtu              int           `json:"mtu"`
	Qdisc            string        `json:"qdisc"`
	Operstate        string        `json:"operstate"`
	Linkmode         string        `json:"linkmode"`
	Group            string        `json:"group"`
	Txqlen           int           `json:"txqlen,omitempty"`
	LinkType         string        `json:"link_type"`
	Address          string        `json:"address"`
	Broadcast        string        `json:"broadcast"`
	Promiscuity      int           `json:"promiscuity"`
	MinMtu           int           `json:"min_mtu"`
	MaxMtu           int           `json:"max_mtu"`
	Inet6AddrGenMode string        `json:"inet6_addr_gen_mode"`
	NumTxQueues      int           `json:"num_tx_queues"`
	NumRxQueues      int           `json:"num_rx_queues"`
	GsoMaxSize       int           `json:"gso_max_size"`
	GsoMaxSegs       int           `json:"gso_max_segs"`
	PhysPortName     string        `json:"phys_port_name,omitempty"`
	PhysSwitchID     string        `json:"phys_switch_id,omitempty"`
	VfinfoList       []interface{} `json:"vfinfo_list,omitempty"`
	PhysPortID       string        `json:"phys_port_id,omitempty"`
	Master           string        `json:"master,omitempty"`
	Linkinfo         struct {
		InfoSlaveKind string `json:"info_slave_kind"`
		InfoKind      string `json:"info_kind"`
	} `json:"linkinfo,omitempty"`
	LinkIndex   int `json:"link_index,omitempty"`
	LinkNetnsid int `json:"link_netnsid,omitempty"`
}

func GetIfs() (macs map[string]export.Iface, macsExist map[string]bool, err error) {
	const (
		ifCommand = "ip -details -json link show"
	)
	stdout, stderr, err := run.LocalCommand(ifCommand)
	if err != nil || stderr != "" {
		return macs, macsExist, fmt.Errorf("could not execute ip command, err=%s stderr=%s", err, stderr)
	}
	macs = make(map[string]export.Iface)
	macsExist = make(map[string]bool)
	aIPOut := []*ipOut{}
	if err := json.Unmarshal([]byte(stdout), &aIPOut); err != nil {
		return macs, macsExist, err
	}
	for _, aIfRaw := range aIPOut {
		if !(aIfRaw.Linkinfo.InfoKind == "" &&
			aIfRaw.LinkType != "loopback") {
			continue
		}
		address, _ := GetPci(aIfRaw.Ifname)
		ptpCaps, _ := GetPtpCaps(aIfRaw.Ifname, run.LocalCommand)
		aIface := export.Iface{IfName: aIfRaw.Ifname, IfMac: export.Mac{Data: strings.ToUpper(aIfRaw.Address)}, IfIndex: aIfRaw.Ifindex, IfPci: address, IfPTPCaps: ptpCaps, IfUp: aIfRaw.Operstate == "UP"}
		macs[aIfRaw.Ifname] = aIface
		macsExist[strings.ToUpper(aIfRaw.Address)] = true
	}
	return macs, macsExist, nil
}

func GetPci(ifaceName string) (aPciAddress export.PCIAddress, err error) {
	const (
		ethtoolBaseCommand = "ethtool -i "
	)
	aCommand := ethtoolBaseCommand + ifaceName
	stdout, stderr, err := run.LocalCommand(aCommand)
	if err != nil || stderr != "" {
		return aPciAddress, fmt.Errorf("could not execute "+ethtoolBaseCommand+" command, err=%s stderr=%s", err, stderr)
	}

	r := regexp.MustCompile(`(?m)bus-info: (.*)\.(\d+)$`)
	for _, submatches := range r.FindAllStringSubmatchIndex(stdout, -1) {
		aPciAddress.Device = string(r.ExpandString([]byte{}, "$1", stdout, submatches))
		aPciAddress.Function = string(r.ExpandString([]byte{}, "$2", stdout, submatches))
	}

	return aPciAddress, nil
}

func GetPtpCaps(ifaceName string, runCmd func(command string) (outStr, errStr string, err error)) (aPTPCaps export.PTPCaps, err error) {
	const (
		ethtoolBaseCommand = "ethtool -T "
		hwTxString         = "hardware-transmit"
		hwRxString         = "hardware-receive"
		hwRawClock         = "hardware-raw-clock"
	)
	aCommand := ethtoolBaseCommand + ifaceName
	stdout, stderr, err := runCmd(aCommand)
	if err != nil || stderr != "" {
		return aPTPCaps, fmt.Errorf("could not execute "+ethtoolBaseCommand+" command, err=%s stderr=%s", err, stderr)
	}

	r := regexp.MustCompile(`(?m)(` + hwTxString + `)|(` + hwRxString + `)|(` + hwRawClock + `)$`)
	for _, submatches := range r.FindAllStringSubmatchIndex(stdout, -1) {
		aString := string(r.ExpandString([]byte{}, "$1", stdout, submatches))
		if !aPTPCaps.HwTx {
			aPTPCaps.HwTx = aString == hwTxString
		}

		aString = string(r.ExpandString([]byte{}, "$2", stdout, submatches))
		if !aPTPCaps.HwRx {
			aPTPCaps.HwRx = aString == hwRxString
		}

		aString = string(r.ExpandString([]byte{}, "$3", stdout, submatches))
		if !aPTPCaps.HwRawClock {
			aPTPCaps.HwRawClock = aString == hwRawClock
		}
	}
	return aPTPCaps, nil
}
