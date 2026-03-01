// Package main provides library examples demonstrating how to use gofixmsg
// to parse and create FIX messages programmatically.
package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/wxcuop/gofixmsg/fixmsg"
	"github.com/wxcuop/gofixmsg/fixmsg/codec"
)

func main() {
	fmt.Println("=== GoFixMsg Library Examples ===\n")

	// Example 1: Create a simple Logon message
	fmt.Println("1. Creating a Logon Message")
	fmt.Println("---------------------------")
	createLogonExample()

	fmt.Println("\n2. Creating a New Order Single Message")
	fmt.Println("-------------------------------------")
	createOrderExample()

	fmt.Println("\n3. Parsing a FIX Message from Wire Format")
	fmt.Println("----------------------------------------")
	parseMessageExample()

	fmt.Println("\n4. Building a Complex Message")
	fmt.Println("----------------------------")
	complexMessageExample()

	fmt.Println("\n5. Working with Message Fields")
	fmt.Println("-----------------------------")
	fieldAccessExample()
}

// Example 1: Create a simple Logon message
func createLogonExample() {
	// Create a new empty message
	msg := fixmsg.NewFixMessage()

	// Set the standard FIX header
	msg.Set(fixmsg.TagBeginString, "FIX.4.4")
	msg.Set(fixmsg.TagMsgType, "A") // 'A' = Logon
	msg.Set(fixmsg.TagSenderCompID, "CLIENT")
	msg.Set(fixmsg.TagTargetCompID, "SERVER")
	msg.Set(fixmsg.TagMsgSeqNum, "1")

	// Set logon-specific fields
	msg.Set(fixmsg.TagHeartBtInt, "30") // 30-second heartbeat

	// Serialize to wire format (includes BodyLength and CheckSum calculation)
	wire, err := msg.ToWire()
	if err != nil {
		log.Fatalf("Serialization failed: %v", err)
	}

	fmt.Printf("Serialized Logon Message:\n")
	fmt.Printf("Wire (hex): %x\n", wire)
	fmt.Printf("Wire (display): %s\n", displayWireFormat(wire))

	// Parse it back to verify
	c := codec.NewNoGroups()
	parsed, err := c.Parse(wire)
	if err != nil {
		log.Fatalf("Parse failed: %v", err)
	}

	fmt.Printf("Parsed back - MsgType: %s, Sender: %s, Target: %s\n",
		parsed.Get(fixmsg.TagMsgType),
		parsed.Get(fixmsg.TagSenderCompID),
		parsed.Get(fixmsg.TagTargetCompID),
	)
}

// Example 2: Create a New Order Single message
func createOrderExample() {
	msg := fixmsg.NewFixMessage()

	// Standard header
	msg.Set(fixmsg.TagBeginString, "FIX.4.4")
	msg.Set(fixmsg.TagMsgType, "D") // 'D' = New Order Single
	msg.Set(fixmsg.TagSenderCompID, "TRADER")
	msg.Set(fixmsg.TagTargetCompID, "EXCHANGE")
	msg.Set(fixmsg.TagMsgSeqNum, "42")

	// Order-specific fields
	msg.Set(fixmsg.TagClOrdID, "ORDER-001")      // Client Order ID
	msg.Set(fixmsg.TagSymbol, "AAPL")             // Stock symbol
	msg.Set(fixmsg.TagSide, "1")                  // '1' = Buy
	msg.Set(fixmsg.TagOrderQty, "1000")           // Quantity
	msg.Set(fixmsg.TagPrice, "150.25")            // Price
	msg.Set(fixmsg.TagOrdType, "2")               // '2' = Limit order
	msg.Set(fixmsg.TagSendingTime, timestampNow()) // Current time

	// Serialize
	wire, err := msg.ToWire()
	if err != nil {
		log.Fatalf("Serialization failed: %v", err)
	}

	fmt.Printf("New Order Single Message:\n")
	fmt.Printf("ClOrdID: %s\n", msg.Get(fixmsg.TagClOrdID))
	fmt.Printf("Symbol: %s\n", msg.Get(fixmsg.TagSymbol))
	fmt.Printf("Side: %s (1=Buy, 2=Sell)\n", msg.Get(fixmsg.TagSide))
	fmt.Printf("Qty: %s @ $%s\n", msg.Get(fixmsg.TagOrderQty), msg.Get(fixmsg.TagPrice))
	fmt.Printf("Wire length: %d bytes\n", len(wire))
}

// Example 3: Parse a FIX message from wire format
func parseMessageExample() {
	// Create a sample execution report message
	rawWire := "8=FIX.4.4|9=120|35=8|49=EXCHANGE|56=TRADER|34=100|52=" +
		timestampNow() + "|11=ORDER-001|37=EXEC-123|39=2|150=2|39=2|" +
		"151=1000|14=500|6=150.25|10=000|\n"

	// Replace | with SOH (0x01)
	wire := []byte(rawWire)
	for i := 0; i < len(wire); i++ {
		if wire[i] == '|' {
			wire[i] = 0x01
		}
	}

	// Parse the message
	c := codec.NewNoGroups()
	msg, err := c.Parse(wire)
	if err != nil {
		log.Fatalf("Parse failed: %v", err)
	}

	// Access fields
	fmt.Printf("Parsed Execution Report:\n")
	fmt.Printf("  MsgType: %s\n", msg.Get(fixmsg.TagMsgType))
	fmt.Printf("  From: %s -> %s (Seq: %s)\n",
		msg.Get(fixmsg.TagSenderCompID),
		msg.Get(fixmsg.TagTargetCompID),
		msg.Get(fixmsg.TagMsgSeqNum),
	)
	fmt.Printf("  ClOrdID: %s\n", msg.Get(fixmsg.TagClOrdID))
	fmt.Printf("  BodyLength: %s\n", msg.Get(fixmsg.TagBodyLength))
	fmt.Printf("  CheckSum: %s\n", msg.Get(fixmsg.TagCheckSum))
}

// Example 4: Building a complex message with multiple fields
func complexMessageExample() {
	msg := fixmsg.NewFixMessage()

	// Using NewFixMessageFromMap for bulk initialization
	fields := map[int]string{
		fixmsg.TagBeginString:  "FIX.4.4",
		fixmsg.TagMsgType:      "D",
		fixmsg.TagSenderCompID: "TRADER1",
		fixmsg.TagTargetCompID: "BROKER",
		fixmsg.TagMsgSeqNum:    "123",
		fixmsg.TagClOrdID:      "CLORD-2024-001",
		fixmsg.TagSymbol:       "MSFT",
		fixmsg.TagSide:         "2", // Sell
		fixmsg.TagOrderQty:     "500",
		fixmsg.TagPrice:        "350.75",
		fixmsg.TagOrdType:      "1", // Market
		58:                     "Test order for example",
	}

	msg = fixmsg.NewFixMessageFromMap(fields)

	// Add more fields if needed
	msg.Set(141, "Y") // ResetSeqNumFlag

	// Show all fields
	fmt.Printf("Complex Message Fields:\n")
	for tag, value := range msg.FixFragment {
		fmt.Printf("  Tag %d: %s\n", tag, value)
	}

	// Serialize
	wire, err := msg.ToWire()
	if err != nil {
		log.Fatalf("Serialization failed: %v", err)
	}
	fmt.Printf("Serialized to %d bytes\n", len(wire))
}

// Example 5: Working with message fields
func fieldAccessExample() {
	msg := fixmsg.NewFixMessage()

	// Set standard header
	msg.Set(fixmsg.TagBeginString, "FIX.4.4")
	msg.Set(fixmsg.TagMsgType, "0") // Heartbeat

	// Different ways to set and get fields
	fmt.Printf("Setting/Getting Field Values:\n")

	// Set by tag number
	msg.Set(49, "SENDER123")
	fmt.Printf("  Tag 49 (SenderCompID): %s\n", msg.Get(49))

	// Check if field exists
	if value, exists := msg.FixFragment[fixmsg.TagSenderCompID]; exists {
		fmt.Printf("  Field exists: %s\n", value)
	}

	// Update a field
	msg.Set(fixmsg.TagTargetCompID, "TARGET456")
	fmt.Printf("  Tag 56 (TargetCompID): %s\n", msg.Get(fixmsg.TagTargetCompID))

	// Iterate over all fields
	fmt.Printf("\n  All fields in message:\n")
	for tag, value := range msg.FixFragment {
		fmt.Printf("    %d: %s\n", tag, value)
	}
}

// Helper function to display wire format with | instead of SOH
func displayWireFormat(wire []byte) string {
	var result string
	for _, b := range wire {
		if b == 0x01 {
			result += "|"
		} else if b >= 32 && b <= 126 {
			result += string(b)
		} else {
			result += "."
		}
	}
	return result
}

// Helper function to get current timestamp in FIX format (YYYYMMDD-HH:MM:SS)
func timestampNow() string {
	return time.Now().UTC().Format("20060102-15:04:05")
}
