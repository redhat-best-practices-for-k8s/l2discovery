package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	//nolint:gocritic
	"unsafe"

	exports "github.com/redhat-cne/l2discovery-exports"
	"github.com/sirupsen/logrus"
)

/*
#include <stdint.h>
#include <stdlib.h>
#include <linux/if_packet.h>
#include <sys/socket.h>

typedef struct __attribute__((packed))
{
    char dest[6];
    char sender[6];
    uint16_t protocolType;
} EthernetHeader;

char* CreateProbe(char* senderMac)
{
    EthernetHeader * packet = malloc(sizeof(EthernetHeader));
    memset(packet, 0, sizeof(EthernetHeader));
    // Ethernet header
    // Dest = Broadcast (ff:ff:ff:ff:ff)
    packet->dest[0] = 0xff;
    packet->dest[1] = 0xff;
    packet->dest[2] = 0xff;
    packet->dest[3] = 0xff;
    packet->dest[4] = 0xff;
    packet->dest[5] = 0xff;

    packet->sender[0] = strtol(senderMac, NULL, 16); senderMac += 3;
    packet->sender[1] = strtol(senderMac, NULL, 16); senderMac += 3;
    packet->sender[2] = strtol(senderMac, NULL, 16); senderMac += 3;
    packet->sender[3] = strtol(senderMac, NULL, 16); senderMac += 3;
    packet->sender[4] = strtol(senderMac, NULL, 16); senderMac += 3;
    packet->sender[5] = strtol(senderMac, NULL, 16);

    packet->protocolType = htons(0x88B5); // local experimental ethertype

    return (char*) packet;
}

int IfaceBind(int fd, int ifindex)
{
	struct sockaddr_ll	sll;
    struct packet_mreq mreq;

	memset(&sll, 0, sizeof(sll));
	memset(&mreq,0,sizeof(mreq));

	sll.sll_family		= AF_PACKET;
	sll.sll_ifindex		= ifindex < 0 ? 0 : ifindex;
	sll.sll_protocol	= 0;

	if (bind(fd, (struct sockaddr *) &sll, sizeof(sll)) == -1) {
		return 1;
	}

	// promiscuous mode needed for PTP
	mreq.mr_ifindex = ifindex;
	mreq.mr_type = PACKET_MR_PROMISC;
	mreq.mr_alen = 6;

	if (setsockopt(fd,SOL_PACKET,PACKET_ADD_MEMBERSHIP,
		(void*)&mreq,(socklen_t)sizeof(mreq)) < 0)
			return -3;
    return 0;
}
*/
import "C" //nolint:gocritic

const (
	bondSlave = "bond"
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
		InfoSlaveData struct {
			State     string `json:"state"`
			MiiStatus string `json:"mii_status"`
		} `json:"info_slave_data"`
	} `json:"linkinfo,omitempty"`
	LinkIndex   int `json:"link_index,omitempty"`
	LinkNetnsid int `json:"link_netnsid,omitempty"`
}

type Frame struct {
	MacDa exports.Mac
	MacSa exports.Mac
	Type  string
}

var (
	MacsPerIface map[string]map[string]*exports.Neighbors
	mu           sync.Mutex
)

func (frame *Frame) parse(rawFrame []byte) {
	frame.MacDa.Data = hex.EncodeToString(rawFrame[0:6])
	frame.MacSa.Data = hex.EncodeToString(rawFrame[6:12])
	frame.Type = hex.EncodeToString(rawFrame[12:14])
}
func (frame *Frame) String() string {
	return fmt.Sprintf("DA=%s SA=%s TYPE=%s", frame.MacDa, frame.MacSa, frame.Type)
}

func RunLocalCommand(command string) (outStr, errStr string, err error) {
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return "", "", err
	}
	outStr, errStr = stdout.String(), stderr.String()
	logrus.Tracef("Command %s, STDERR: %s, STDOUT: %s", cmd.String(), errStr, outStr)
	return outStr, errStr, err
}

func main() {
	logrus.SetLevel(logrus.FatalLevel)
	macs, macExist, _ := getIfs()
	MacsPerIface = make(map[string]map[string]*exports.Neighbors)
	for _, iface := range macs {
		RecordAllLocal(iface)
		go RecvFrame(iface, macExist)
		if iface.IfSlaveType != bondSlave {
			go sendProbeForever(iface)
		}
	}
	go PrintLog()
	select {}
}

func sendProbe(iface *exports.Iface) {
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, syscall.ETH_P_ALL)
	defer syscall.Close(fd)
	if err != nil {
		logrus.Errorf("Error: " + err.Error())
		return
	}
	// for Link aggregation interfaces, use the link aggregated interface to send the probe packets
	// The bond interface will carry it in a way so as to not generate traffic loops. As a result,
	// only the primary port, responsible to carry broadcast and multicast traffic will be discovered by
	// l2discovery
	senderIface := iface.IfName
	if iface.IfSlaveType == bondSlave {
		senderIface = iface.IfMaster
	}

	err = syscall.BindToDevice(fd, senderIface)
	if err != nil {
		panic(err)
	}
	C.IfaceBind(C.int(fd), C.int(iface.IfIndex))
	ether := new(C.EthernetHeader)
	size := uint(unsafe.Sizeof(*ether))
	logrus.Tracef("Size : %d", size)
	interf, err := net.InterfaceByName(senderIface)
	if err != nil {
		logrus.Errorf("Could not find " + senderIface + " interface")
		return
	}
	logrus.Tracef("Interface hw address: %s", iface.IfMac)

	ifaceCstr := C.CString(iface.IfMac.Data)

	packet := C.GoBytes(unsafe.Pointer(C.CreateProbe(ifaceCstr)), C.int(size))

	// Send the packet
	var addr syscall.SockaddrLinklayer
	addr.Protocol = syscall.ETH_P_ARP
	addr.Ifindex = interf.Index
	addr.Hatype = syscall.ARPHRD_ETHER
	err = syscall.Sendto(fd, packet, 0, &addr)

	if err != nil {
		logrus.Errorf("error: %s", err)
	}
	logrus.Tracef("Sent packet")
}

func RecvFrame(iface *exports.Iface, macsExist map[string]bool) {
	const (
		recvTimeout           = 2
		recvBufferSize        = 1024
		experimentalEthertype = "88b5"
		ptpEthertype          = "88f7"
		allEthPacketTypes     = 0x0300
	)
	time.Sleep(time.Second * recvTimeout)
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, allEthPacketTypes)
	defer syscall.Close(fd)
	if err != nil {
		syscall.Close(fd)
		panic(err)
	}

	C.IfaceBind(C.int(fd), C.int(iface.IfIndex))

	data := make([]byte, recvBufferSize)
	for {
		_, _, err := syscall.Recvfrom(fd, data, 0)
		if err != nil {
			continue
		}
		var aFrame Frame
		aFrame.parse(data)
		mu.Lock()
		// only processes experimental and ptp frames
		if strings.EqualFold(aFrame.Type, experimentalEthertype) || strings.EqualFold(aFrame.Type, ptpEthertype) {
			if _, ok := macsExist[strings.ToUpper(aFrame.MacSa.String())]; !ok {
				if _, ok := MacsPerIface[aFrame.Type]; !ok {
					MacsPerIface[aFrame.Type] = make(map[string]*exports.Neighbors)
				}
				if _, ok := MacsPerIface[aFrame.Type][iface.IfName]; !ok {
					aNeighbors := exports.Neighbors{Local: *iface, Remote: make(map[string]bool)}
					MacsPerIface[aFrame.Type][iface.IfName] = &aNeighbors
				}
				if MacsPerIface[aFrame.Type][iface.IfName].Local.IfMac != aFrame.MacSa {
					MacsPerIface[aFrame.Type][iface.IfName].Remote[aFrame.MacSa.String()] = true
				}
			}
		}
		mu.Unlock()
	}
}

func RecordAllLocal(iface *exports.Iface) {
	const (
		localInterfaces = "0000"
	)
	mu.Lock()
	if _, ok := MacsPerIface[localInterfaces]; !ok {
		MacsPerIface[localInterfaces] = make(map[string]*exports.Neighbors)
	}
	if _, ok := MacsPerIface[localInterfaces][iface.IfName]; !ok {
		aNeighbors := exports.Neighbors{Local: *iface, Remote: make(map[string]bool)}
		MacsPerIface[localInterfaces][iface.IfName] = &aNeighbors
	}
	mu.Unlock()
}

func PrintLog() {
	const logPrintPeriod = 5 // in seconds
	for {
		mu.Lock()
		aString, err := json.Marshal(MacsPerIface)
		if err != nil {
			logrus.Errorf("Cannot marshall MacsPerIface")
		}
		// Only log printed
		fmt.Printf("JSON_REPORT%s\n", string(aString))
		mu.Unlock()
		time.Sleep(time.Second * logPrintPeriod)
	}
}

func sendProbeForever(iface *exports.Iface) {
	// sending probe frames mess with link aggregation. After sending a maximum number of probes for a given
	// interface, stop forever. Discovery should be complete by then.
	const maxProbes = 10
	for i := 0; i < maxProbes; i++ {
		time.Sleep(time.Second * 1)
		sendProbe(iface)
	}
}

func getIfs() (macs map[string]*exports.Iface, macsExist map[string]bool, err error) {
	const (
		ifCommand = "ip -details -json link show"
	)
	stdout, stderr, err := runLocalCommand(ifCommand)
	if err != nil || stderr != "" {
		return macs, macsExist, fmt.Errorf("could not execute ip command, err=%s stderr=%s", err, stderr)
	}
	macs = make(map[string]*exports.Iface)
	macsExist = make(map[string]bool)
	aIPOut := []*ipOut{}
	err = json.Unmarshal([]byte(stdout), &aIPOut)
	if err != nil {
		return macs, macsExist, err
	}
	for _, aIfRaw := range aIPOut {
		if aIfRaw.LinkType == "loopback" ||
			aIfRaw.Linkinfo.InfoKind != "" {
			continue
		}
		address, _ := getPci(aIfRaw.Ifname)
		ptpCaps, _ := getPtpCaps(aIfRaw.Ifname, runLocalCommand)
		aIface := exports.Iface{IfName: aIfRaw.Ifname,
			IfMac:   exports.Mac{Data: strings.ToUpper(aIfRaw.Address)},
			IfIndex: aIfRaw.Ifindex,
			IfPci:   address, IfPTPCaps: ptpCaps,
			IfUp:        aIfRaw.Operstate == "UP",
			IfMaster:    aIfRaw.Master,
			IfSlaveType: aIfRaw.Linkinfo.InfoSlaveKind}
		macs[aIfRaw.Ifname] = &aIface
		macsExist[strings.ToUpper(aIfRaw.Address)] = true
	}
	return macs, macsExist, nil
}

func getPci(ifaceName string) (aPciAddress exports.PCIAddress, err error) {
	const (
		ethtoolBaseCommand  = "ethtool -i"
		lscpiCommand        = "lspci -vv -s"
		newLineCharacter    = "\n"
		emptySpaceSeparator = " "
		subsystemString     = "Subsystem: "
	)
	aCommand := fmt.Sprintf("%s %s", ethtoolBaseCommand, ifaceName)
	stdout, stderr, err := RunLocalCommand(aCommand)
	if err != nil || stderr != "" {
		return aPciAddress, fmt.Errorf("could not execute ethtool command, err=%s stderr=%s", err, stderr)
	}

	r := regexp.MustCompile(`(?m)bus-info: (.*)\.(\d+)$`)
	for _, submatches := range r.FindAllStringSubmatchIndex(stdout, -1) {

		aPciAddress.Device = string(r.ExpandString([]byte{}, "$1", stdout, submatches))
		aPciAddress.Function = string(r.ExpandString([]byte{}, "$2", stdout, submatches))
	}

	aCommand = fmt.Sprintf("%s %s.%s", lscpiCommand, aPciAddress.Device, aPciAddress.Function)
	stdout, stderr, err = RunLocalCommand(aCommand)
	if err != nil || stderr != "" {
		return aPciAddress, fmt.Errorf("could not execute lspci command, err=%s stderr=%s", err, stderr)
	}

	description, subsystem, err := parseLspci(stdout)
	if err != nil {
		return aPciAddress, fmt.Errorf("could not parse lspci output, err=%s", err)
	}
	aPciAddress.Description = description
	aPciAddress.Subsystem = subsystem

	return aPciAddress, nil
}

func parseLspci(output string) (description, subsystem string, err error) {
	const regex = `(?m)\S*\s*(.*)$(?m)\s+Subsystem:\s*(.*)$`

	// Compile the regular expression
	re := regexp.MustCompile(regex) //nolint:gocritic

	// Find all matches
	matches := re.FindAllStringSubmatch(output, -1)

	if len(matches) < 1 {
		return description, subsystem, fmt.Errorf("could not parse lspci output")
	}
	description = matches[0][1]
	subsystem = matches[0][2]
	return description, subsystem, nil
}

func getPtpCaps(ifaceName string, runCmd func(command string) (outStr, errStr string, err error)) (aPTPCaps exports.PTPCaps, err error) {
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

func runLocalCommand(command string) (outStr, errStr string, err error) {
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return "", "", err
	}
	outStr, errStr = stdout.String(), stderr.String()
	logrus.Tracef("Command %s, STDERR: %s, STDOUT: %s", cmd.String(), errStr, outStr)
	return outStr, errStr, err
}
