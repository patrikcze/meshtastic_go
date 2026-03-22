package serial

import (
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"strings"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

const (
	portSpeed = 115200 //921600
	dataBits  = 8
)

// GetPorts returns a list of serial ports that are likely to be Meshtastic devices
func GetPorts() []string {
	slog.Info("Searching for Meshtastic devices...")

	// Get a list of all available ports
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		slog.Error("Failed to get port list", "error", err)
		return nil
	}

	if len(ports) == 0 {
		slog.Info("No serial ports found on system")
		return nil
	}

	slog.Info("Found serial ports", "count", len(ports))

	// Print all discovered ports for debugging
	for i, port := range ports {
		if port.IsUSB {
			slog.Debug("Found USB device",
				"index", i,
				"name", port.Name,
				"VID", port.VID,
				"PID", port.PID,
				"product", port.Product)
		} else {
			slog.Debug("Found non-USB port",
				"index", i,
				"name", port.Name)
		}
	}

	matches := classifyPorts(ports, runtime.GOOS)
	for _, match := range matches {
		slog.Info("Detected Meshtastic device", "port", match.Name, "reasons", strings.Join(match.Reasons, ", "))
	}

	meshtasticPorts := make([]string, 0, len(matches))
	for _, match := range matches {
		meshtasticPorts = append(meshtasticPorts, match.Name)
	}

	if len(meshtasticPorts) > 0 {
		slog.Info("Found Meshtastic devices", "count", len(meshtasticPorts))
		slog.Debug("Detected ports", "ports", meshtasticPorts)
	}

	return meshtasticPorts
}

// Connect opens a connection to a serial port with the appropriate settings for Meshtastic
func Connect(port string) (serial.Port, error) {
	slog.Info("Connecting to port", "port", port)
	slog.Debug("Opening connection to port", "port", port)
	mode := &serial.Mode{
		BaudRate: portSpeed,
		DataBits: dataBits,
	}

	p, err := serial.Open(port, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port %s: %w", port, err)
	}

	slog.Info("Connected to port", "port", port)
	return p, nil
}

type portMatch struct {
	Name       string
	Reasons    []string
	Confidence int
}

func classifyPorts(ports []*enumerator.PortDetails, goos string) []portMatch {
	matches := make([]portMatch, 0, len(ports))

	for _, port := range ports {
		portName := port.Name
		reasons := []string{}
		confidence := 0

		if port.IsUSB {
			slog.Debug("Checking USB device for VID/PID match", "port", portName, "VID", port.VID, "PID", port.PID)
			if isKnownDevice(port.VID, port.PID) {
				confidence = 2
				reasons = append(reasons, fmt.Sprintf("matches known VID/PID: %s:%s", port.VID, port.PID))
			}
		}

		if confidence == 0 {
			if reason := platformPortReason(portName, goos); reason != "" {
				confidence = 1
				reasons = append(reasons, reason)
			}
		}

		if confidence == 0 {
			slog.Debug("Ignoring port", "port", portName, "reason", "no match for Meshtastic criteria")
			continue
		}

		matches = append(matches, portMatch{
			Name:       portName,
			Reasons:    reasons,
			Confidence: confidence,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Confidence != matches[j].Confidence {
			return matches[i].Confidence > matches[j].Confidence
		}
		return matches[i].Name < matches[j].Name
	})

	return matches
}

func isKnownDevice(vid, pid string) bool {
	for _, device := range knownDevices {
		if device.VID == vid && device.PID == pid {
			return true
		}
	}
	return false
}

func platformPortReason(portName, goos string) string {
	lowerName := strings.ToLower(portName)

	switch goos {
	case "darwin":
		if strings.Contains(lowerName, "usbmodem") {
			return "matches usbmodem pattern"
		}
	case "linux":
		if strings.Contains(lowerName, "ttyusb") || strings.Contains(lowerName, "ttyacm") {
			return "matches tty pattern"
		}
	case "windows":
		if strings.Contains(lowerName, "com") && !strings.Contains(lowerName, "bluetooth") {
			return "matches COM pattern"
		}
	}

	return ""
}
