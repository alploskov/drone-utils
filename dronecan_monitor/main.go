package main

import (
	"fmt"
	"strings"
	"github.com/bluenviron/gomavlib/v3"
    "github.com/bluenviron/gomavlib/v3/pkg/dialects/ardupilotmega"
)

type Message struct {
    ID      uint32
    Content string
}

var messages []Message

// CAN ID types
type MessageCANID struct {
	Priority       uint8
	MessageTypeID  uint16
	SourceNodeID   uint8
}

type AnonymousMessageCANID struct {
	Priority               uint8
	Discriminator          uint16
	LowerBitsMessageTypeID uint8
}

type ServiceCANID struct {
	Priority          uint8
	ServiceTypeID     uint8
	IsRequest         bool
	DestinationNodeID uint8
	SourceNodeID      uint8
}

func parseServiceCANID(canID uint32) ServiceCANID {
	return ServiceCANID{
		Priority:          uint8((canID >> 24) & 0x1F),
		ServiceTypeID:     uint8((canID >> 16) & 0xFF),
		IsRequest:         (canID>>15)&0x1 == 1,
		DestinationNodeID: uint8((canID >> 8) & 0x7F),
		SourceNodeID:      uint8(canID & 0x7F),
	}
}

func parseAnonymousMessageCANID(canID uint32) AnonymousMessageCANID {
	return AnonymousMessageCANID{
		Priority:               uint8((canID >> 24) & 0x1F),
		Discriminator:          uint16((canID >> 10) & 0x3FFF),
		LowerBitsMessageTypeID: uint8((canID >> 8) & 0x3),
	}
}

func parseMessageCANID(canID uint32) MessageCANID {
	return MessageCANID{
		Priority:      uint8((canID >> 24) & 0x1F),
		MessageTypeID: uint16((canID >> 8) & 0xFFFF),
		SourceNodeID:  uint8(canID & 0x7F),
	}
}

// Output functions for each CAN ID type
func formatServiceOutput(svc ServiceCANID) (string, string, string) {
	return fmt.Sprintf("%d", svc.SourceNodeID),
		fmt.Sprintf("SVC:%d", svc.ServiceTypeID),
		fmt.Sprintf("%d", svc.DestinationNodeID)
}

func formatMessageOutput(msg MessageCANID) (string, string, string) {
	return fmt.Sprintf("%d", msg.SourceNodeID),
		fmt.Sprintf("MSG:%d", msg.MessageTypeID),
		"nan"
}

func formatAnonymousOutput(anon AnonymousMessageCANID) (string, string, string) {
	return "0 (anon)",
		fmt.Sprintf("ANON:%d", anon.Discriminator),
		"nan"
}

// Print table header
func printTableHeader() {
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("| %-10s | %-12s | %-10s |\n", "source_node", "data_type_id", "dest_node")
	fmt.Println(strings.Repeat("-", 40))
}

// Print table row
func printTableRow(source, dataType, dest string) {
	fmt.Printf("| %-10s | %-12s | %-10s |\n", source, dataType, dest)
}

// Print table footer
func printTableFooter() {
	fmt.Println(strings.Repeat("-", 40))
}

// Process single CAN ID and print its row
func processCANID(canID uint32) {
	serviceNotMessage := (canID >> 7) & 0x1
	var source, dataType, dest string

	if serviceNotMessage == 1 {
		svc := parseServiceCANID(canID)
		source, dataType, dest = formatServiceOutput(svc)
	} else {
		sourceNodeID := uint8(canID & 0x7F)
		if sourceNodeID == 0 {
			anon := parseAnonymousMessageCANID(canID)
			source, dataType, dest = formatAnonymousOutput(anon)
		} else {
			msg := parseMessageCANID(canID)
			source, dataType, dest = formatMessageOutput(msg)
		}
	}
	printTableRow(source, dataType, dest)
}

func main() {

	node, err := gomavlib.NewNode(gomavlib.NodeConf{
		Endpoints: []gomavlib.EndpointConf{
			gomavlib.EndpointSerial{
				Device: "/dev/ttyACM0",
				Baud:   57600,
			},
		},
		Dialect:     ardupilotmega.Dialect,
		OutVersion:  gomavlib.V2, // change to V1 if you're unable to communicate with the target
		OutSystemID: 10,
	})
	if err != nil {
        panic(err)
    }
    defer node.Close()

	var targetSystemID uint8
	for evt := range node.Events() {
		if frm, ok := evt.(*gomavlib.EventFrame); ok {
			if _, ok := frm.Message().(*ardupilotmega.MessageHeartbeat); ok {
				targetSystemID = frm.SystemID()
				fmt.Printf("Received heartbeat from system %d\n", targetSystemID)
				break
			}
		}
	}

	cmd := &ardupilotmega.MessageCommandLong{
		TargetSystem:    targetSystemID,
		TargetComponent: 1, // Usually 1 for autopilot
		Command:        32000,//ardupilotmega.MAV_CMD_NAV_TAKEOFF,
		Confirmation:   0,
		Param1:         2,
		Param2:         0,
		Param3:         0,
		Param4:         0,
		Param5:         0,
		Param6:         0,
		Param7:         0,
	}

	fmt.Printf("Sending command: %+v\n", cmd)
	node.WriteMessageAll(cmd)

	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("| %-10s | %-12s | %-10s |\n", "source_node", "data_type_id", "dest_node")
	fmt.Println(strings.Repeat("-", 40))
	
	for evt := range node.Events() {
        if frm, ok := evt.(*gomavlib.EventFrame); ok {
			if frame, ok := frm.Message().(*ardupilotmega.MessageCanFrame); ok {
				if (frame.Data[frame.Len - 1] & 0b10000000 != 0) { // Starts of transfer only
					processCANID(frame.Id)
				}
			}
			if _, ok := frm.Message().(*ardupilotmega.MessageHeartbeat); ok {
				node.WriteMessageAll(cmd)
			}
        }
    }
}
