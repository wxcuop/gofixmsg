// Package main demonstrates advanced usage patterns with FixSpec customization.
// This example mirrors the Python pyfixmsg spec customization capabilities.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wxcuop/gofixmsg/fixmsg"
	"github.com/wxcuop/gofixmsg/fixmsg/codec"
	"github.com/wxcuop/gofixmsg/fixmsg/spec"
)

func main() {
	fmt.Println("=== GoFixMsg Advanced Spec Usage ===")

	// Load FIX spec
	specFile := findSpecFile("FIX44.xml")
	var fixSpec *spec.FixSpec
	if specFile != "" {
		s, err := spec.Load(specFile)
		if err != nil {
			log.Printf("Warning: Failed to load FIX spec: %v\n", err)
		} else {
			fixSpec = s
			fmt.Printf("Loaded FIX spec from %s (version %s)\n\n", specFile, fixSpec.Version)
		}
	} else {
		fmt.Println("Note: FIX44.xml not found; examples will use basic codec")
	}

	// Example 1: Standard codec usage
	fmt.Println("1. Standard Codec Usage")
	fmt.Println("---------------------")
	standardCodecExample(fixSpec)

	fmt.Println("\n2. Message Serialization and Deserialization")
	fmt.Println("-------------------------------------------")
	serdeExample(fixSpec)

	fmt.Println("\n3. Field Type Handling")
	fmt.Println("--------------------")
	fieldTypeExample(fixSpec)

	fmt.Println("\n4. Building Messages from Scratch")
	fmt.Println("--------------------------------")
	buildMessageExample(fixSpec)
}

// findSpecFile searches for FIX44.xml in common locations
func findSpecFile(filename string) string {
	// Search in order: current dir, parent (gofixmsg), two levels up
	searchPaths := []string{
		filename,
		filepath.Join("..", filename),
		filepath.Join("..", "..", filename),
	}

	for _, path := range searchPaths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// Example 1: Standard codec usage
func standardCodecExample(fixSpec *spec.FixSpec) {
	// Create a codec with spec (if available)
	var noSpecCodec *codec.Codec
	if fixSpec != nil {
		noSpecCodec = codec.New(fixSpec)
		fmt.Printf("Using FIX spec %s for parsing\n", fixSpec.Version)
	} else {
		noSpecCodec = codec.NewNoGroups()
		fmt.Printf("No spec available; using basic codec")
	}

	// Sample message
	data := "8=FIX.4.4|9=50|35=A|49=SENDER|56=TARGET|108=30|10=123|"
	wire := []byte(data)
	for i := 0; i < len(wire); i++ {
		if wire[i] == '|' {
			wire[i] = 0x01
		}
	}

	// Parse
	msg, err := noSpecCodec.Parse(wire)
	if err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	fmt.Printf("Parsed message with NoGroups codec:\n")
	fmt.Printf("  MsgType: %s\n", msg.MustGet(fixmsg.TagMsgType))
	fmt.Printf("  SenderCompID: %s\n", msg.MustGet(fixmsg.TagSenderCompID))
	fmt.Printf("  HeartBtInt: %s\n", msg.MustGet(fixmsg.TagHeartBtInt))

	// Note: In Go, unlike Python with FixSpec, the basic codec does not carry
	// metadata about message types or field definitions. To add that level of
	// specification, you would need to use a FixSpec (which requires loading a QuickFIX spec file).
	fmt.Printf("\nNote: For full spec support with message type validation and field metadata,\n")
	fmt.Printf("you would need to load a QuickFIX .xml spec file using fixmsg/spec/FixSpec.\n")
	fmt.Printf("Currently, the Go implementation focuses on core parsing/serialization.\n")
}

// Example 2: Message serialization and deserialization
func serdeExample(fixSpec *spec.FixSpec) {
	// Create a message from scratch
	original := fixmsg.NewFixMessage()
	original.Set(fixmsg.TagBeginString, "FIX.4.4")
	original.Set(fixmsg.TagMsgType, "A")
	original.Set(fixmsg.TagSenderCompID, "TRADER")
	original.Set(fixmsg.TagTargetCompID, "EXCHANGE")
	original.Set(fixmsg.TagMsgSeqNum, "1")
	original.Set(fixmsg.TagHeartBtInt, "30")

	// Serialize to wire format
	wire, err := original.ToWire()
	if err != nil {
		log.Fatalf("Serialization error: %v", err)
	}

	fmt.Printf("Serialized message length: %d bytes\n", len(wire))
	fmt.Printf("Wire format (with . for non-printable):\n  ")

	// Display wire format with special chars escaped
	for _, b := range wire {
		if b == 0x01 {
			fmt.Print("|")
		} else if b >= 32 && b <= 126 {
			fmt.Printf("%c", b)
		} else {
			fmt.Printf("<%02x>", b)
		}
	}
	fmt.Println()

	// Deserialize back
	parsed := fixmsg.NewFixMessage()
	err = parsed.LoadFix(wire)
	if err != nil {
		log.Fatalf("Deserialization error: %v", err)
	}

	fmt.Printf("\nDeserialized message:\n")
	fmt.Printf("  MsgType: %s\n", parsed.MustGet(fixmsg.TagMsgType))
	fmt.Printf("  SenderCompID: %s\n", parsed.MustGet(fixmsg.TagSenderCompID))
	fmt.Printf("  TargetCompID: %s\n", parsed.MustGet(fixmsg.TagTargetCompID))

	// Verify roundtrip
	fmt.Printf("\nRoundtrip validation:\n")
	allMatch := true
	for tag, origValue := range original.FixFragment {
		if parsedValue, exists := parsed.Get(tag); !exists || parsedValue != origValue {
			allMatch = false
			fmt.Printf("  Tag %d mismatch: %s vs %s\n", tag, origValue, parsedValue)
		}
	}
	if allMatch {
		fmt.Printf("  All fields match after roundtrip ✓\n")
	}
}

// Example 3: Field type handling
func fieldTypeExample(fixSpec *spec.FixSpec) {
	msg := fixmsg.NewFixMessage()

	// In gofixmsg, like pyfixmsg, all fields are stored as strings internally
	msg.Set(fixmsg.TagMsgType, "D")
	msg.Set(fixmsg.TagSenderCompID, "TRADER")
	msg.Set(38, "1000")        // OrderQty
	msg.Set(44, "150.25")      // Price
	msg.Set(54, "1")           // Side

	fmt.Printf("Field values and their Go types in storage:\n")
	msgType, _ := msg.Get(fixmsg.TagMsgType)
	qty, _ := msg.Get(38)
	price, _ := msg.Get(44)
	side, _ := msg.Get(54)

	fmt.Printf("  MsgType '%s': stored as string\n", msgType)
	fmt.Printf("  OrderQty '%s': stored as string (numeric handling by application)\n", qty)
	fmt.Printf("  Price '%s': stored as string (numeric handling by application)\n", price)
	fmt.Printf("  Side '%s': stored as string (enum handling by application)\n", side)

	fmt.Printf("\nNote: Applications are responsible for type conversion and validation.\n")
	fmt.Printf("Unlike Python pyfixmsg with FixSpec, Go version doesn't auto-convert types.\n")

	// Demonstrate comparisons
	fmt.Printf("\nField comparisons:\n")
	fmt.Printf("  Price == '150.25': %v\n", msg.TagExact(44, "150.25", false))
	fmt.Printf("  Price == '150.00': %v\n", msg.TagExact(44, "150.00", false))
	fmt.Printf("  OrderQty > '500': %v (string comparison)\n", qty > "500")
}

// Example 4: Building complex messages
func buildMessageExample(fixSpec *spec.FixSpec) {
	// Create a New Order Single message
	order := fixmsg.NewFixMessage()

	// Standard header
	order.Set(fixmsg.TagBeginString, "FIX.4.4")
	order.Set(fixmsg.TagMsgType, "D") // New Order Single

	// Session identifiers
	order.Set(fixmsg.TagSenderCompID, "TRADER")
	order.Set(fixmsg.TagTargetCompID, "EXCHANGE")
	order.Set(fixmsg.TagMsgSeqNum, "42")

	// Client order info
	order.Set(fixmsg.TagClOrdID, "ORDER-2024-001")
	order.Set(fixmsg.TagSendingTime, "20240301-12:00:00")

	// Order details
	order.Set(55, "AAPL")        // Symbol
	order.Set(54, "1")           // Side (1=Buy)
	order.Set(38, "1000")        // OrderQty
	order.Set(40, "2")           // OrdType (2=Limit)
	order.Set(44, "150.25")      // Price
	order.Set(59, "0")           // TimeInForce (0=Day)

	// Additional fields
	order.Set(1, "ACCOUNT123")   // Account
	order.Set(47, "A")           // PartyRole

	fmt.Printf("Created New Order Single message:\n")
	fmt.Printf("  ClOrdID: %s\n", order.MustGet(fixmsg.TagClOrdID))
	fmt.Printf("  Symbol: %s\n", order.MustGet(55))
	fmt.Printf("  Side: %s (1=Buy)\n", order.MustGet(54))
	fmt.Printf("  Qty: %s\n", order.MustGet(38))
	fmt.Printf("  Type: %s (2=Limit)\n", order.MustGet(40))
	fmt.Printf("  Price: $%s\n", order.MustGet(44))
	fmt.Printf("  Account: %s\n", order.MustGet(1))

	// Serialize
	wire, err := order.ToWire()
	if err != nil {
		log.Fatalf("Serialization failed: %v", err)
	}

	fmt.Printf("\nSerialized to %d bytes\n", len(wire))

	// Show field count
	fmt.Printf("Total fields in message: %d\n", len(order.FixFragment))

	// Demonstrate bulk field update
	fmt.Printf("\nUpdating multiple fields:\n")
	updates := map[int]string{
		54: "2",           // Change to Sell
		44: "151.00",      // Update price
		59: "1",           // Change TimeInForce
	}
	for tag, value := range updates {
		order.Set(tag, value)
		fmt.Printf("  Tag %d -> %s\n", tag, value)
	}

	// Verify updates
	fmt.Printf("\nVerified updates:\n")
	fmt.Printf("  Side now: %s (2=Sell)\n", order.MustGet(54))
	fmt.Printf("  Price now: %s\n", order.MustGet(44))
}

// ============================================================================
// Note on Spec Customization
// ============================================================================
// The original Python example shows customizing the FixSpec by:
// 1. Adding custom tags (spec.tags.add_tag(10001, "MyTagName"))
// 2. Adding enum values to existing tags
// 3. Adding repeating groups to message types
//
// In the Go implementation, this is partially supported through the
// fixmsg/spec package, which can load QuickFIX .xml spec files.
// However, runtime customization like the Python example would require
// modifying the FixSpec structures directly.
//
// Example of what would be possible (pseudo-code):
//
//    mySpec := spec.NewFixSpec()
//    mySpec.AddTag(10001, "MyTagName", "string")
//    mySpec.Tags[54].AddEnumValue("SF", "SELLFAST")
//
// For now, the focus is on parsing and serializing messages correctly,
// with the assumption that message definitions come from standard specs.
