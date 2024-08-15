package main

import "testing"

func Test_parseLspci(t *testing.T) {
	type args struct {
		output string
	}
	tests := []struct {
		name        string
		args        args
		exDesc      string
		exSubsystem string
		parseErr    bool
	}{
		{
			name: "ok",
			args: args{
				output: "ca:00.0 Ethernet controller: Intel Corporation Ethernet Controller E810-C for SFP (rev 02)\n" +
					"Subsystem: Intel Corporation Ethernet Network Adapter E810-XXV-4T\nControl: I/O- Mem+ BusMaster+ " +
					"SpecCycle- MemWINV- VGASnoop- ParErr- Stepping- SERR- FastB2B- DisINTx+\n",
			},
			exDesc:      "Ethernet controller: Intel Corporation Ethernet Controller E810-C for SFP (rev 02)",
			exSubsystem: "Intel Corporation Ethernet Network Adapter E810-XXV-4T",
			parseErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, subsystem, err := parseLspci(tt.args.output)
			if (err != nil) != tt.parseErr {
				t.Errorf("parseLspci() error = %v, wantErr %v", err, tt.parseErr)
				return
			}
			if desc != tt.exDesc {
				t.Errorf("parseLspci() description does not match got = %v, want %v", desc, tt.exDesc)
			}
			if subsystem != tt.exSubsystem {
				t.Errorf("parseLspci() subsystem does not match got = %v, want %v", subsystem, tt.exSubsystem)
			}
		})
	}
}
