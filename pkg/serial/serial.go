package serial

import (
	"fmt"
	"log/slog"
	"runtime"
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

	// Filter for likely Meshtastic devices
	var meshtasticPorts []string

	// Check for platform-specific patterns
	for _, port := range ports {
		portName := port.Name
		reasons := []string{}
		isLikelyMeshtastic := false

		// First check for pattern matches in port name
		switch runtime.GOOS {
		case "darwin":
			if strings.Contains(strings.ToLower(portName), "usbmodem") {
				isLikelyMeshtastic = true
				reasons = append(reasons, "matches usbmodem pattern")
			}
		case "linux":
			if strings.Contains(strings.ToLower(portName), "ttyusb") ||
				strings.Contains(strings.ToLower(portName), "ttyacm") {
				isLikelyMeshtastic = true
				reasons = append(reasons, "matches tty pattern")
			}
		case "windows":
			if strings.Contains(strings.ToLower(portName), "com") &&
				!strings.Contains(strings.ToLower(portName), "bluetooth") {
				isLikelyMeshtastic = true
				reasons = append(reasons, "matches COM pattern")
			}
		}

		// Then check USB devices by VID/PID
		if port.IsUSB {
			slog.Debug("Checking USB device for VID/PID match", "port", portName, "VID", port.VID, "PID", port.PID)

			// Check against knownDevices list
			for _, device := range knownDevices {
				if device.VID == port.VID && device.PID == port.PID {
					isLikelyMeshtastic = true
					reasons = append(reasons, fmt.Sprintf("matches known VID/PID: %s:%s", port.VID, port.PID))
					break
				}
			}
		}

		// Add the port if it matches any criteria
		if isLikelyMeshtastic {
			slog.Info("Detected Meshtastic device", "port", portName, "reasons", strings.Join(reasons, ", "))
			meshtasticPorts = append(meshtasticPorts, portName)
		} else {
			slog.Debug("Ignoring port", "port", portName, "reason", "no match for Meshtastic criteria")
		}
	}

	// If we found any devices, return them
	if len(meshtasticPorts) > 0 {
		slog.Info("Found Meshtastic devices", "count", len(meshtasticPorts))
		slog.Debug("Detected ports", "ports", meshtasticPorts)
		return meshtasticPorts
	}

	// Last resort: include any serial port with "usb" in the name
	// This is a fallback for devices that don't match our known patterns
	for _, port := range ports {
		if strings.Contains(strings.ToLower(port.Name), "usb") {
			slog.Warn("No Meshtastic devices found, trying fallback detection", "port", port.Name)
			meshtasticPorts = append(meshtasticPorts, port.Name)
		}
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
