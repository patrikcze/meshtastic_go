// pkg/meshtastic/portnums.go
package meshtastic

// PortNum represents the different port numbers in the Meshtastic protocol.
type PortNum int32

// Enum values for PortNum.
const (
	PortNum_UNKNOWN_APP                 PortNum = 0
	PortNum_TEXT_MESSAGE_APP            PortNum = 1
	PortNum_REMOTE_HARDWARE_APP         PortNum = 2
	PortNum_POSITION_APP                PortNum = 3
	PortNum_NODEINFO_APP                PortNum = 4
	PortNum_ROUTING_APP                 PortNum = 5
	PortNum_ADMIN_APP                   PortNum = 6
	PortNum_TEXT_MESSAGE_COMPRESSED_APP PortNum = 7
	PortNum_WAYPOINT_APP                PortNum = 8
	PortNum_AUDIO_APP                   PortNum = 9
	PortNum_DETECTION_SENSOR_APP        PortNum = 10
	PortNum_REPLY_APP                   PortNum = 32
	PortNum_IP_TUNNEL_APP               PortNum = 33
	PortNum_PAXCOUNTER_APP              PortNum = 34
	PortNum_SERIAL_APP                  PortNum = 64
	PortNum_STORE_FORWARD_APP           PortNum = 65
	PortNum_RANGE_TEST_APP              PortNum = 66
	PortNum_TELEMETRY_APP               PortNum = 67
	PortNum_ZPS_APP                     PortNum = 68
	PortNum_SIMULATOR_APP               PortNum = 69
	PortNum_TRACEROUTE_APP              PortNum = 70
	PortNum_NEIGHBORINFO_APP            PortNum = 71
	PortNum_ATAK_PLUGIN                 PortNum = 72
	PortNum_MAP_REPORT_APP              PortNum = 73
	PortNum_POWERSTRESS_APP             PortNum = 74
	PortNum_PRIVATE_APP                 PortNum = 256
	PortNum_ATAK_FORWARDER              PortNum = 257
	PortNum_MAX                         PortNum = 511
)
