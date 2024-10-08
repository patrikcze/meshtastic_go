package protocol

import (
	"fmt"
	"log"
	"meshtastic_go/pkg/generated"
	"os"
	"text/tabwriter"
)

// HandleChannel processes and logs channel data in a structured way.
func HandleChannel(channel *generated.Channel) {
	channels := append([]*generated.Channel{}, channel)
	PrintChannelInfoTable(channels)
}

// PrintChannelInfoTable prints all channel information in a tabular format.
func PrintChannelInfoTable(channels []*generated.Channel) {
	if len(channels) == 0 {
		return // No channels to print
	}

	// Create a new tabwriter for nicely formatted table output
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print the table header with borders
	fmt.Fprintln(writer, "┌──────┬──────────────────┬─────────┬───────────┬──────────────────────┐")
	fmt.Fprintln(writer, "│ Idx  │ Name             │ Uplink  │ Downlink  │ Position Precision   │")
	fmt.Fprintln(writer, "├──────┼──────────────────┼─────────┼───────────┼──────────────────────┤")

	// Iterate over the channels and print their details with borders
	for _, channel := range channels {
		settings := channel.GetSettings()
		moduleSettings := settings.GetModuleSettings()
		fmt.Fprintf(writer, "│ %-4d │ %-16s │ %-7t │ %-9t │ %-20d │\n",
			channel.GetIndex(),
			settings.GetName(),
			settings.GetUplinkEnabled(),
			settings.GetDownlinkEnabled(),
			moduleSettings.GetPositionPrecision(),
		)
	}

	// Print the table footer after all channels have been printed
	fmt.Fprintln(writer, "└──────┴──────────────────┴─────────┴───────────┴──────────────────────┘")

	// Flush the writer to ensure the output is printed
	if err := writer.Flush(); err != nil {
		// Handle the error appropriately, e.g., log it
		log.Printf("Failed to flush writer: %v", err)
	}
}
