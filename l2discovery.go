package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

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
import "C"

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

type Mac struct {
	Data string
}

type Iface struct {
	IfName  string
	IfMac   Mac
	IfIndex int
}

type Neighbors struct {
	Local  Iface
	Remote map[string]bool
}
type Frame struct {
	MacDa Mac
	MacSa Mac
	Type  string
}

var (
	MacsPerIface map[string]map[string]*Neighbors
	mu           sync.Mutex
)

func (mac Mac) String() string {
	return strings.ToUpper(string([]byte(mac.Data)[0:2]) + ":" +
		string([]byte(mac.Data)[2:4]) + ":" +
		string([]byte(mac.Data)[4:6]) + ":" +
		string([]byte(mac.Data)[6:8]) + ":" +
		string([]byte(mac.Data)[8:10]) + ":" +
		string([]byte(mac.Data)[10:12]))
}

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
	macs, macExist, _ := getIfs()
	MacsPerIface = make(map[string]map[string]*Neighbors)
	for _, iface := range macs {
		go RecvFrame(iface, macExist)
		go sendProbeForever(iface)
	}
	go PrintLog()
	select {}
}

func sendProbe(iface Iface) {
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, syscall.ETH_P_ALL)
	defer syscall.Close(fd)
	if err != nil {
		fmt.Println("Error: " + err.Error())
		return
	}
	err = syscall.BindToDevice(fd, iface.IfName)
	if err != nil {
		panic(err)
	}
	C.IfaceBind(C.int(fd), C.int(iface.IfIndex))
	ether := new(C.EthernetHeader)
	size := uint(unsafe.Sizeof(*ether))
	logrus.Tracef("Size : %d", size)
	interf, err := net.InterfaceByName(iface.IfName)
	if err != nil {
		fmt.Println("Could not find " + iface.IfName + " interface")
		return
	}
	logrus.Tracef("Interface hw address: %s", iface.IfMac)

	iface_cstr := C.CString(iface.IfMac.Data)

	packet := C.GoBytes(unsafe.Pointer(C.CreateProbe(iface_cstr)), C.int(size))

	// Send the packet
	var addr syscall.SockaddrLinklayer
	addr.Protocol = syscall.ETH_P_ARP
	addr.Ifindex = interf.Index
	addr.Hatype = syscall.ARPHRD_ETHER
	err = syscall.Sendto(fd, packet, 0, &addr)

	if err != nil {
		fmt.Println("Error: ", err)
	}
	logrus.Tracef("Sent packet")
}

func RecvFrame(iface Iface, macsExist map[string]bool) {
	const (
		recvTimeout           = 2
		recvBufferSize        = 1024
		experimentalEthertype = "88b5"
		ptpEthertype 		  = "88f7"
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

		}
		var aFrame Frame
		aFrame.parse(data)
		mu.Lock()
		// only processes experimental and ptp frames
		if strings.EqualFold(aFrame.Type, experimentalEthertype) || strings.EqualFold(aFrame.Type, ptpEthertype) {
			if _, ok := macsExist[strings.ToUpper(aFrame.MacSa.String())]; !ok {
				if _, ok := MacsPerIface[aFrame.Type]; !ok {
					MacsPerIface[aFrame.Type] = make(map[string]*Neighbors)
				}
				if _, ok := MacsPerIface[aFrame.Type][iface.IfName]; !ok {
					aNeighbors := Neighbors{Local: iface, Remote: make(map[string]bool)}
					MacsPerIface[aFrame.Type][iface.IfName] = &aNeighbors
				}
				MacsPerIface[aFrame.Type][iface.IfName].Remote[aFrame.MacSa.String()] = true
			}
		}
		mu.Unlock()
	}
}

func PrintLog() {
	const logPrintPeriod = 5 // in seconds
	for {
		mu.Lock()
		aString, err := json.Marshal(MacsPerIface)
		if err != nil {
			fmt.Println("Cannot marshall MacsPerIface")
		}
		fmt.Printf("JSON_REPORT%s\n", string(aString))
		mu.Unlock()
		time.Sleep(time.Second * logPrintPeriod)
	}
}

func sendProbeForever(iface Iface) {
	for {
		sendProbe(iface)
		time.Sleep(time.Second * 1)
	}
}

func getIfs() (macs map[string]Iface, macsExist map[string]bool, err error) {
	const (
		ifCommand = "ip -details -json link show"
	)
	stdout, stderr, err := RunLocalCommand(ifCommand)
	if err != nil || stderr != "" {
		return macs, macsExist, fmt.Errorf("could not execute ip command, err=%s stderr=%s", err, stderr)
	}
	macs = make(map[string]Iface)
	macsExist = make(map[string]bool)
	aIpOut := []*ipOut{}
	if err := json.Unmarshal([]byte(stdout), &aIpOut); err != nil {
		return macs, macsExist, err
	}
	for _, aIfRaw := range aIpOut {
		if aIfRaw.Operstate == "UP" &&
			aIfRaw.Linkinfo.InfoKind == "" &&
			aIfRaw.LinkType != "loopback" {
			aIface := Iface{IfName: aIfRaw.Ifname, IfMac: Mac{Data:strings.ToUpper(aIfRaw.Address)}, IfIndex: aIfRaw.Ifindex}
			macs[aIfRaw.Ifname] = aIface
			macsExist[strings.ToUpper(aIfRaw.Address)] = true
		}
	}
	return macs, macsExist, nil
}
