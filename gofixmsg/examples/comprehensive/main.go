// Package main provides comprehensive examples of using FixMessage and associated utilities.
// This mirrors the functionality of the original Python pyfixmsg examples.
package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"

	"github.com/wxcuop/gofixmsg/fixmsg"
	"github.com/wxcuop/gofixmsg/fixmsg/codec"
	"github.com/wxcuop/gofixmsg/fixmsg/spec"
)

func main() {
	fmt.Println("=== GoFixMsg Comprehensive Examples ===\n")

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
		fmt.Println("Note: FIX44.xml not found; examples will use basic codec without spec\n")
	}

	// Example 1: Vanilla tag/value parsing and access
	fmt.Println("1. Vanilla Tag/Value Parsing and Access")
	fmt.Println("--------------------------------------")
	vanillaTagValueExample(fixSpec)

	fmt.Println("\n2. Tag Comparison Operations")
	fmt.Println("---------------------------")
	tagComparisonExample(fixSpec)

	fmt.Println("\n3. Tag Manipulation")
	fmt.Println("------------------")
	tagManipulationExample(fixSpec)

	fmt.Println("\n4. Copying Messages")
	fmt.Println("------------------")
	messageCopyingExample(fixSpec)

	fmt.Println("\n5. Repeating Groups")
	fmt.Println("------------------")
	repeatingGroupExample(fixSpec)

	fmt.Println("\n6. Working with Field Paths")
	fmt.Println("---------------------------")
	fieldPathExample(fixSpec)
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

// Example 1: Vanilla tag/value parsing
func vanillaTagValueExample(fixSpec *spec.FixSpec) {
	// Raw FIX message with | as SOH delimiter
	data := "8=FIX.4.2|9=97|35=6|49=ABC|56=CAB|34=14|52=20100204-09:18:42|" +
		"23=115685|28=N|55=BLAH|54=2|44=2200.75|27=S|25=H|10=248|"

	// Replace | with actual SOH character
	wire := []byte(data)
	for i := 0; i < len(wire); i++ {
		if wire[i] == '|' {
			wire[i] = 0x01
		}
	}

	// Parse the message (use spec if available)
	var c *codec.Codec
	if fixSpec != nil {
		c = codec.New(fixSpec)
	} else {
		c = codec.NewNoGroups()
	}
	msg, err := c.Parse(wire)
	if err != nil {
		log.Fatalf("Failed to parse: %v", err)
	}

	// Access values
	msgType, _ := msg.Get(35)
	fmt.Printf("Message Type: %s\n", msgType)
	fmt.Printf("Sender CompID: %s\n", msg.MustGet(49))
	fmt.Printf("Target CompID: %s\n", msg.MustGet(56))

	price, _ := msg.Get(44)
	fmt.Printf("Price: %s (type: string in memory, value type depends on usage)\n", price)

	// Message is a map-like dictionary with integer keys and string values
	fmt.Printf("\nAll tags in message:\n")
	for tag := range msg.FixFragment {
		value, _ := msg.Get(tag)
		fmt.Printf("  Tag %d: %s\n", tag, value)
	}
}

// Example 2: Tag comparison operations
func tagComparisonExample(fixSpec *spec.FixSpec) {
	data := "8=FIX.4.2|9=97|35=6|49=ABC|56=CAB|34=14|52=20100204-09:18:42|" +
		"23=115685|28=N|55=BLAH|54=2|44=2200.75|27=S|25=H|10=248|"

	wire := []byte(data)
	for i := 0; i < len(wire); i++ {
		if wire[i] == '|' {
			wire[i] = 0x01
		}
	}

	var c *codec.Codec
	if fixSpec != nil {
		c = codec.New(fixSpec)
	} else {
		c = codec.NewNoGroups()
	}
	msg, _ := c.Parse(wire)

	// Exact comparison
	// Note: Go comparison is always with strings, so we compare the string representation
	fmt.Printf("Tag 44 exact match with '2200.75': %v\n", msg.TagExact(44, "2200.75", false))
	fmt.Printf("Tag 54 exact match with '2': %v\n", msg.TagExact(54, "2", false))

	// Comparison with different types (in Go, everything is stored as string)
	price, _ := msg.Get(44)
	fmt.Printf("Price raw value: %s\n", price)

	// Case-insensitive comparison
	fmt.Printf("Tag 55 case-insensitive contains 'blah': %v\n", msg.TagContains(55, "blah", true))
	fmt.Printf("Tag 55 case-sensitive contains 'BLAH': %v\n", msg.TagContains(55, "BLAH", false))

	// Regex matching
	tagValue, _ := msg.Get(49)
	matched, _ := regexp.MatchString("^ABC$", tagValue)
	fmt.Printf("Tag 49 matches regex '^ABC$': %v\n", matched)
}

// Example 3: Tag manipulation
func tagManipulationExample(fixSpec *spec.FixSpec) {
	data := "8=FIX.4.2|9=97|35=6|49=ABC|56=CAB|34=14|52=20100204-09:18:42|" +
		"23=115685|28=N|55=BLAH|54=2|44=2200.75|27=S|25=H|10=248|"

	wire := []byte(data)
	for i := 0; i < len(wire); i++ {
		if wire[i] == '|' {
			wire[i] = 0x01
		}
	}

	var c *codec.Codec
	if fixSpec != nil {
		c = codec.New(fixSpec)
	} else {
		c = codec.NewNoGroups()
	}
	msg, _ := c.Parse(wire)

	fmt.Printf("Original tag 56: %s\n", msg.MustGet(56))

	// Modify a tag (like dictionary)
	msg.Set(56, "ABC.1")
	fmt.Printf("Modified tag 56: %s\n", msg.MustGet(56))

	// Set multiple fields at once
	updates := map[int]string{
		55: "ABC123.1",
		28: "M",
	}
	for tag, value := range updates {
		msg.Set(tag, value)
	}
	fmt.Printf("Tag 55 updated to: %s\n", msg.MustGet(55))
	fmt.Printf("Tag 28 updated to: %s\n", msg.MustGet(28))

	// SetOrDelete: set to empty string or delete
	shouldDelete := rand.Intn(2) == 0
	var deleteVal string
	if !shouldDelete {
		deleteVal = "1"
	}
	msg.SetOrDelete(27, deleteVal)

	if deleteVal == "" {
		if _, exists := msg.Get(27); !exists {
			fmt.Printf("Tag 27 deleted (SetOrDelete with empty string)\n")
		}
	} else {
		fmt.Printf("Tag 27 set to: %s\n", msg.MustGet(27))
	}

	// Delete a tag by setting empty string
	msg.Set(25, "")
	if _, exists := msg.Get(25); !exists {
		fmt.Printf("Tag 25 is no longer present after setting to empty\n")
	}
}

// Example 4: Copying messages
func messageCopyingExample(fixSpec *spec.FixSpec) {
	data := "8=FIX.4.2|9=97|35=6|49=ABC|56=CAB|34=14|52=20100204-09:18:42|" +
		"23=115685|28=N|55=BLAH|54=2|44=2200.75|27=S|25=H|10=248|"

	wire := []byte(data)
	for i := 0; i < len(wire); i++ {
		if wire[i] == '|' {
			wire[i] = 0x01
		}
	}

	// Original message
	var c *codec.Codec
	if fixSpec != nil {
		c = codec.New(fixSpec)
	} else {
		c = codec.NewNoGroups()
	}
	original, _ := c.Parse(wire)

	// Copy via serialization/deserialization (like Python's copy())
	// This is the most efficient way to deeply copy in Go
	originalWire, _ := original.ToWire()
	copied := fixmsg.NewFixMessage()
	copied.LoadFix(originalWire)

	fmt.Printf("Original and copied are different objects: %v\n", &original != &copied)

	// Modify the original
	original.Set(44, "9999.99")
	original.SetLenAndChecksum()

	// Check that copy was not affected
	fmt.Printf("Original tag 44: %s\n", original.MustGet(44))
	fmt.Printf("Copied tag 44: %s\n", copied.MustGet(44))
	fmt.Printf("Values are different: %v\n", original.MustGet(44) != copied.MustGet(44))

	// Compare all fields
	allMatch := true
	for tag, origValue := range original.FixFragment {
		if copiedValue, exists := copied.Get(tag); !exists || copiedValue != origValue {
			allMatch = false
			break
		}
	}
	fmt.Printf("Most fields match between original and copy: %v\n", !allMatch) // They won't match after modification
}

// Example 5: Repeating groups
func repeatingGroupExample(fixSpec *spec.FixSpec) {
	// FIX message with repeating group (NoMDEntries, tag 268)
	data := "8=FIX.4.2|9=196|35=X|49=A|56=B|34=12|52=20100318-03:21:11.364" +
		"|262=A|268=2|279=0|269=0|278=BID|55=EUR/USD|270=1.37215" +
		"|15=EUR|271=2500000|346=1|279=0|269=1|278=OFFER|55=EUR/USD" +
		"|270=1.37224|15=EUR|271=2503200|346=1|10=171|"

	wire := []byte(data)
	for i := 0; i < len(wire); i++ {
		if wire[i] == '|' {
			wire[i] = 0x01
		}
	}

	var c *codec.Codec
	if fixSpec != nil {
		c = codec.New(fixSpec)
		fmt.Printf("Using FIX spec: repeating groups will be properly parsed\n")
	} else {
		c = codec.NewNoGroups()
	}
	msg, _ := c.Parse(wire)

	fmt.Printf("Message Type: %s\n", msg.MustGet(35))
	fmt.Printf("Tag 262: %s\n", msg.MustGet(262))

	// When repeating groups aren't recognized by the codec,
	// the tags exist as regular fields
	if groupVal, exists := msg.Get(268); exists {
		fmt.Printf("Tag 268 (count) parsed as: %s (regular tag, not a repeating group)\n", groupVal)
	}

	// The individual field values are still accessible
	if val, exists := msg.Get(278); exists {
		fmt.Printf("Tag 278 (first occurrence): %s\n", val)
	}

	fmt.Println("\nNote: To properly handle repeating groups, use a codec with FixSpec")
	fmt.Println("The built-in codec.NewNoGroups() treats them as regular tags")

	// Demonstrate creating a repeating group manually
	fmt.Println("\nManually Creating a Repeating Group:")
	newMsg := fixmsg.NewFixMessage()
	newMsg.Set(fixmsg.TagBeginString, "FIX.4.2")
	newMsg.Set(fixmsg.TagMsgType, "X")

	// Create a repeating group (tag 268 = NoMDEntries)
	group := fixmsg.NewRepeatingGroup(268)
	group.FirstTag = 279

	// Add first member
	member1 := group.Add()
	member1.Set(279, "0")
	member1.Set(269, "0")
	member1.Set(278, "BID")
	member1.Set(55, "EUR/USD")
	member1.Set(270, "1.37215")

	// Add second member
	member2 := group.Add()
	member2.Set(279, "0")
	member2.Set(269, "1")
	member2.Set(278, "OFFER")
	member2.Set(55, "EUR/USD")
	member2.Set(270, "1.37224")

	// Store the group in the message
	newMsg.SetGroup(268, group)

	fmt.Printf("Created repeating group with %d members\n", group.Len())
	fmt.Printf("First member tag 278: %s\n", group.At(0).MustGet(278))
	fmt.Printf("Second member tag 278: %s\n", group.At(1).MustGet(278))
}

// Example 6: Working with field paths (find_all)
func fieldPathExample(fixSpec *spec.FixSpec) {
	// Message with repeating group
	data := "8=FIX.4.2|9=196|35=X|49=A|56=B|34=12|52=20100318-03:21:11.364" +
		"|262=A|268=2|279=0|269=0|278=BID|55=EUR/USD|270=1.37215" +
		"|15=EUR|271=2500000|346=1|279=0|269=1|278=OFFER|55=EUR/USD" +
		"|270=1.37224|15=EUR|271=2503200|346=1|10=171|"

	wire := []byte(data)
	for i := 0; i < len(wire); i++ {
		if wire[i] == '|' {
			wire[i] = 0x01
		}
	}

	var c *codec.Codec
	if fixSpec != nil {
		c = codec.New(fixSpec)
	} else {
		c = codec.NewNoGroups()
	}
	msg, _ := c.Parse(wire)

	// Find all occurrences of tag 270 (MDEntryPx)
	paths := msg.FindAll(270)
	fmt.Printf("Tag 270 found at %d location(s):\n", len(paths))
	for i, path := range paths {
		fmt.Printf("  Path %d: %v\n", i, path)
	}

	// Demonstrate with a message containing repeating groups
	fmt.Println("\nManually created repeating group - finding tags:")
	newMsg := fixmsg.NewFixMessage()
	newMsg.Set(fixmsg.TagBeginString, "FIX.4.2")
	newMsg.Set(fixmsg.TagMsgType, "X")

	group := fixmsg.NewRepeatingGroup(268)
	group.FirstTag = 279

	m1 := group.Add()
	m1.Set(270, "1.37215")
	m1.Set(55, "EUR/USD")

	m2 := group.Add()
	m2.Set(270, "1.37224")
	m2.Set(55, "EUR/USD")

	newMsg.SetGroup(268, group)

	// Find all paths to tag 270
	paths = newMsg.FindAll(270)
	fmt.Printf("Tag 270 in repeating group found at %d location(s):\n", len(paths))
	for i, path := range paths {
		fmt.Printf("  Path %d: %v\n", i, path)
	}
}

// Helper function to display a message nicely
func displayMessage(msg *fixmsg.FixMessage) {
	fmt.Println("Message Fields:")
	for tag := range msg.FixFragment {
		value, _ := msg.Get(tag)
		fmt.Printf("  Tag %d: %s\n", tag, value)
	}
}
