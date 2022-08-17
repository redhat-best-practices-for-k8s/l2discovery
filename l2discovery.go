package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/test-network-function/l2discovery/l2lib/pkg/export"
	"github.com/test-network-function/l2discovery/pkg/cgo"
	"github.com/test-network-function/l2discovery/pkg/concur"
	"github.com/test-network-function/l2discovery/pkg/ifacehelper"
)

var (
	MacsPerIface map[string]map[string]*export.Neighbors
)

func RecordAllLocal(iface export.Iface) {
	const (
		localInterfaces = "0000"
	)
	concur.Mu.Lock()
	if _, ok := MacsPerIface[localInterfaces]; !ok {
		MacsPerIface[localInterfaces] = make(map[string]*export.Neighbors)
	}
	if _, ok := MacsPerIface[localInterfaces][iface.IfName]; !ok {
		aNeighbors := export.Neighbors{Local: iface, Remote: make(map[string]bool)}
		MacsPerIface[localInterfaces][iface.IfName] = &aNeighbors
	}
	concur.Mu.Unlock()
}

func PrintLog() {
	const logPrintPeriod = 5 // in seconds
	for {
		concur.Mu.Lock()
		aString, err := json.Marshal(MacsPerIface)
		if err != nil {
			fmt.Println("Cannot marshall MacsPerIface")
		}
		fmt.Printf("JSON_REPORT%s\n", string(aString))
		concur.Mu.Unlock()
		time.Sleep(time.Second * logPrintPeriod)
	}
}

func main() {
	macs, macExist, _ := ifacehelper.GetIfs()
	MacsPerIface = make(map[string]map[string]*export.Neighbors)
	for _, iface := range macs {
		RecordAllLocal(iface)
		go cgo.RecvFrame(iface, macExist, MacsPerIface)
		go cgo.SendProbeForever(iface)
	}
	go PrintLog()
	select {}
}
