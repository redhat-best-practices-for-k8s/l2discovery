package ifacehelper

import (
	"reflect"
	"testing"

	"github.com/test-network-function/l2discovery/l2lib/pkg/export"
)

func TestGetPtpCaps(t *testing.T) {
	type args struct {
		ifaceName string
		runCmd    func(command string) (outStr, errStr string, err error)
	}
	tests := []struct {
		name         string
		args         args
		wantAPTPCaps export.PTPCaps
		wantErr      bool
	}{
		{
			name:         "ok",
			args:         args{ifaceName: "enp0s31f6", runCmd: RunLocalCommandTestOk},
			wantAPTPCaps: export.PTPCaps{HwRx: true, HwTx: true, HwRawClock: true},
			wantErr:      false,
		},
		{
			name:         "nok",
			args:         args{ifaceName: "enp0s31f6", runCmd: RunLocalCommandTestNok},
			wantAPTPCaps: export.PTPCaps{HwRx: true, HwTx: false, HwRawClock: true},
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAPTPCaps, err := GetPtpCaps(tt.args.ifaceName, tt.args.runCmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPtpCaps() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotAPTPCaps, tt.wantAPTPCaps) {
				t.Errorf("GetPtpCaps() = %v, want %v", gotAPTPCaps, tt.wantAPTPCaps)
			}
		})
	}
}

func RunLocalCommandTestOk(command string) (outStr, errStr string, err error) {
	return ` ethtool -T enp0s31f6
	Time stamping parameters for enp0s31f6:
	Capabilities:
		hardware-transmit
		software-transmit
		hardware-receive
		software-receive
		software-system-clock
		hardware-raw-clock
	PTP Hardware Clock: 0
	Hardware Transmit Timestamp Modes:
		off
		on
	Hardware Receive Filter Modes:
		none
		all
		ptpv1-l4-sync
		ptpv1-l4-delay-req
		ptpv2-l4-sync
		ptpv2-l4-delay-req
		ptpv2-l2-sync
		ptpv2-l2-delay-req
		ptpv2-event
		ptpv2-sync
		ptpv2-delay-req
	`, "", nil
}

func RunLocalCommandTestNok(command string) (outStr, errStr string, err error) {
	return ` ethtool -T enp0s31f6
	Time stamping parameters for enp0s31f6:
	Capabilities:
		software-transmit
		hardware-receive
		software-receive
		software-system-clock
		hardware-raw-clock
	PTP Hardware Clock: 0
	Hardware Transmit Timestamp Modes:
		off
		on
	Hardware Receive Filter Modes:
		none
		all
		ptpv1-l4-sync
		ptpv1-l4-delay-req
		ptpv2-l4-sync
		ptpv2-l4-delay-req
		ptpv2-l2-sync
		ptpv2-l2-delay-req
		ptpv2-event
		ptpv2-sync
		ptpv2-delay-req
	`, "", nil
}
