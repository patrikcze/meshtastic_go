package serial

import (
	"testing"

	"go.bug.st/serial/enumerator"
)

func TestClassifyPortsPrefersKnownDevices(t *testing.T) {
	ports := []*enumerator.PortDetails{
		{Name: "/dev/ttyUSB0", IsUSB: false},
		{Name: "/dev/ttyACM0", IsUSB: true, VID: "239A", PID: "8029"},
	}

	got := classifyPorts(ports, "linux")

	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	if got[0].Name != "/dev/ttyACM0" {
		t.Fatalf("expected known VID/PID match first, got %q", got[0].Name)
	}
	if got[0].Confidence != 2 {
		t.Fatalf("expected high-confidence match, got %d", got[0].Confidence)
	}
}

func TestClassifyPortsRejectsLooseUSBFallback(t *testing.T) {
	ports := []*enumerator.PortDetails{
		{Name: "/dev/usbserial-random", IsUSB: false},
	}

	got := classifyPorts(ports, "linux")
	if len(got) != 0 {
		t.Fatalf("expected no matches for loose usb fallback, got %+v", got)
	}
}

func TestPlatformPortReason(t *testing.T) {
	tests := []struct {
		name     string
		portName string
		goos     string
		want     string
	}{
		{name: "darwin usbmodem", portName: "/dev/cu.usbmodem101", goos: "darwin", want: "matches usbmodem pattern"},
		{name: "linux ttyusb", portName: "/dev/ttyUSB0", goos: "linux", want: "matches tty pattern"},
		{name: "windows com", portName: "COM4", goos: "windows", want: "matches COM pattern"},
		{name: "windows bluetooth", portName: "Bluetooth COM4", goos: "windows", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := platformPortReason(tt.portName, tt.goos)
			if got != tt.want {
				t.Fatalf("platformPortReason(%q, %q) = %q, want %q", tt.portName, tt.goos, got, tt.want)
			}
		})
	}
}
