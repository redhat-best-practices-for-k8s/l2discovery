package cgo

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
	"unsafe" //nolint:gocritic

	"github.com/sirupsen/logrus"
	"github.com/test-network-function/l2discovery/l2lib/pkg/export"
	"github.com/test-network-function/l2discovery/pkg/concur"
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

type Frame struct {
	MacDa export.Mac
	MacSa export.Mac
	Type  string
}

func (frame *Frame) parse(rawFrame []byte) {
	frame.MacDa.Data = hex.EncodeToString(rawFrame[0:6])
	frame.MacSa.Data = hex.EncodeToString(rawFrame[6:12])
	frame.Type = hex.EncodeToString(rawFrame[12:14])
}

func (frame *Frame) String() string {
	return fmt.Sprintf("DA=%s SA=%s TYPE=%s", frame.MacDa, frame.MacSa, frame.Type)
}

func SendProbe(iface export.Iface) {
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
	ether := new(C.EthernetHeader) //nolint:staticcheck
	size := uint(unsafe.Sizeof(*ether))
	logrus.Tracef("Size : %d", size)
	interf, err := net.InterfaceByName(iface.IfName)
	if err != nil {
		fmt.Println("Could not find " + iface.IfName + " interface")
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
		fmt.Println("Error: ", err)
	}
	logrus.Tracef("Sent packet")
}

func RecvFrame(iface export.Iface, macsExist map[string]bool, macsPerIface map[string]map[string]*export.Neighbors) {
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
		concur.Mu.Lock()
		// only processes experimental and ptp frames
		if strings.EqualFold(aFrame.Type, experimentalEthertype) || strings.EqualFold(aFrame.Type, ptpEthertype) {
			if _, ok := macsPerIface[aFrame.Type]; !ok {
				macsPerIface[aFrame.Type] = make(map[string]*export.Neighbors)
			}
			if _, ok := macsPerIface[aFrame.Type][iface.IfName]; !ok {
				aNeighbors := export.Neighbors{Local: iface, Remote: make(map[string]bool)}
				macsPerIface[aFrame.Type][iface.IfName] = &aNeighbors
			}
			if macsPerIface[aFrame.Type][iface.IfName].Local.IfMac != aFrame.MacSa {
				macsPerIface[aFrame.Type][iface.IfName].Remote[aFrame.MacSa.String()] = true
			}
		}
		concur.Mu.Unlock()
	}
}

func SendProbeForever(iface export.Iface) {
	for {
		SendProbe(iface)
		time.Sleep(time.Second * 1)
	}
}
