package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/wxcuop/gofixmsg/fixmsg"
	"github.com/wxcuop/gofixmsg/fixmsg/codec"
)

func main() {
	var (
		msgType    = flag.String("type", "", "Message type (e.g., D=NewOrderSingle, A=Logon)")
		sender     = flag.String("sender", "SENDER", "Sender CompID")
		target     = flag.String("target", "TARGET", "Target CompID")
		seqNum     = flag.Int("seq", 1, "Message sequence number")
		fields     = flag.String("fields", "", "Additional fields as tag=value,tag=value...")
		prettyPrint = flag.Bool("pretty", false, "Pretty-print the output")
		output     = flag.String("output", "", "Write to file (if empty, write to stdout)")
		ascii      = flag.Bool("ascii", false, "Output with | instead of SOH for readability")
	)
	flag.Parse()

	if *msgType == "" {
		fmt.Fprintf(os.Stderr, "Usage: create-message -type <type> [-sender <id>] [-target <id>] [-fields <tag=val,...>] [-pretty] [-ascii] [-output <file>]\n")
		fmt.Fprintf(os.Stderr, "\nExample: create-message -type D -sender SENDER -target TARGET -fields \"55=AAPL,54=1,40=2,38=100,44=150.25\"\n")
		fmt.Fprintf(os.Stderr, "\nCommon message types:\n")
		fmt.Fprintf(os.Stderr, "  A = Logon\n")
		fmt.Fprintf(os.Stderr, "  D = NewOrderSingle\n")
		fmt.Fprintf(os.Stderr, "  8 = Execution Report\n")
		fmt.Fprintf(os.Stderr, "  0 = Heartbeat\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create a new FIX message
	msg := fixmsg.NewFixMessage()

	// Set standard header tags
	msg.Set(fixmsg.TagBeginString, "FIX.4.4")
	msg.Set(fixmsg.TagMsgType, *msgType)
	msg.Set(fixmsg.TagSenderCompID, *sender)
	msg.Set(fixmsg.TagTargetCompID, *target)
	msg.Set(fixmsg.TagMsgSeqNum, strconv.Itoa(*seqNum))

	// Parse and set additional fields
	if *fields != "" {
		fieldPairs := strings.Split(*fields, ",")
		for _, pair := range fieldPairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				log.Fatalf("Invalid field format: %s (expected tag=value)", pair)
			}
			tag, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				log.Fatalf("Invalid tag number: %s", parts[0])
			}
			value := strings.TrimSpace(parts[1])
			msg.Set(tag, value)
		}
	}

	// Serialize the message
	wire, err := msg.ToWire()
	if err != nil {
		log.Fatalf("Failed to serialize message: %v", err)
	}

	// Output
	if *prettyPrint {
		fmt.Println("Generated FIX Message:")
		fmt.Println("======================")
		printMessageDetails(msg)
		fmt.Println("\nWire Format (with | for SOH):")
		fmt.Println(formatWireReadable(wire))
	} else if *ascii {
		// Replace SOH with | for readability
		output := formatWireReadable(wire)
		fmt.Print(output)
	} else {
		// Write raw bytes
		os.Stdout.Write(wire)
	}

	// Write to file if specified
	if *output != "" {
		if err := os.WriteFile(*output, wire, 0644); err != nil {
			log.Fatalf("Failed to write to file: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Message written to %s\n", *output)
	}
}

func printMessageDetails(msg *fixmsg.FixMessage) {
	tags := extractAndSortTags(msg)
	for _, tag := range tags {
		value := msg.FixFragment[tag]
		fmt.Printf("  Tag %d (%-20s): %s\n", tag, tagName(tag), value)
	}
}

func formatWireReadable(wire []byte) string {
	var sb strings.Builder
	for i, b := range wire {
		if b == 0x01 {
			sb.WriteString("|")
		} else if b >= 32 && b < 127 {
			sb.WriteByte(b)
		} else {
			sb.WriteString(fmt.Sprintf("\\x%02x", b))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func extractAndSortTags(msg *fixmsg.FixMessage) []int {
	tags := make([]int, 0, len(msg.FixFragment))
	for tag := range msg.FixFragment {
		tags = append(tags, tag)
	}
	// Simple numeric sort
	for i := 0; i < len(tags); i++ {
		for j := i + 1; j < len(tags); j++ {
			if tags[j] < tags[i] {
				tags[i], tags[j] = tags[j], tags[i]
			}
		}
	}
	return tags
}

var tagNames = map[int]string{
	8:   "BeginString",
	9:   "BodyLength",
	10:  "CheckSum",
	11:  "ClOrdID",
	34:  "MsgSeqNum",
	35:  "MsgType",
	40:  "OrdType",
	43:  "PossDupFlag",
	44:  "Price",
	49:  "SenderCompID",
	52:  "SendingTime",
	54:  "Side",
	55:  "Symbol",
	56:  "TargetCompID",
	58:  "Text",
	108: "HeartBtInt",
	112: "TestReqID",
	122: "OrigSendTime",
	141: "ResetSeqNumFlag",
	372: "RefMsgType",
	373: "SessionRejectReason",
}

func tagName(tag int) string {
	if name, ok := tagNames[tag]; ok {
		return name
	}
	return "Field"
}
