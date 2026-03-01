package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/wxcuop/gofixmsg/fixmsg"
	"github.com/wxcuop/gofixmsg/fixmsg/codec"
)

func main() {
	var (
		input      = flag.String("input", "", "FIX message (wire format with SOH as '|' or raw with actual SOH)")
		file       = flag.String("file", "", "File containing FIX message")
		prettyPrint = flag.Bool("pretty", false, "Pretty-print output")
		tagInfo    = flag.Bool("tags", false, "Show tag numbers and names")
	)
	flag.Parse()

	if *input == "" && *file == "" {
		fmt.Fprintf(os.Stderr, "Usage: parse -input <msg> [-pretty] [-tags]\n")
		fmt.Fprintf(os.Stderr, "   or: parse -file <path> [-pretty] [-tags]\n")
		fmt.Fprintf(os.Stderr, "\nNote: Use '|' to represent SOH character in -input flag\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	var buf []byte
	var err error

	if *file != "" {
		buf, err = os.ReadFile(*file)
		if err != nil {
			log.Fatalf("Failed to read file: %v", err)
		}
	} else {
		// Replace | with actual SOH character (0x01)
		buf = []byte(*input)
		for i := 0; i < len(buf); i++ {
			if buf[i] == '|' {
				buf[i] = 0x01
			}
		}
	}

	// Parse the message using the codec
	c := codec.NewNoGroups()
	msg, err := c.Parse(buf)
	if err != nil {
		log.Fatalf("Failed to parse FIX message: %v", err)
	}

	if *prettyPrint {
		printPretty(msg, *tagInfo)
	} else {
		printCompact(msg, *tagInfo)
	}
}

func printCompact(msg *fixmsg.FixMessage, showTags bool) {
	for tag, value := range msg.FixFragment {
		if showTags {
			fmt.Printf("%d (%s): %s\n", tag, tagName(tag), value)
		} else {
			fmt.Printf("%d: %s\n", tag, value)
		}
	}
}

func printPretty(msg *fixmsg.FixMessage, showTags bool) {
	// Sort tags in canonical FIX order
	tags := extractAndSortTags(msg)

	fmt.Println("+-------+------------------+--------------------------------------------+")
	if showTags {
		fmt.Println("| Tag   | Name             | Value                                      |")
	} else {
		fmt.Println("| Tag   | Value                                          |")
	}
	fmt.Println("+-------+------------------+--------------------------------------------+")

	for _, tag := range tags {
		value := fmt.Sprint(msg.FixFragment[tag])
		if showTags {
			name := tagName(tag)
			fmt.Printf("| %-5d | %-16s | %-40s |\n", tag, name, truncate(value, 40))
		} else {
			fmt.Printf("| %-5d | %-40s |\n", tag, truncate(value, 40))
		}
	}
	fmt.Println("+-------+------------------+--------------------------------------------+")
}

func extractAndSortTags(msg *fixmsg.FixMessage) []int {
	tags := make([]int, 0, len(msg.FixFragment))
	for tag := range msg.FixFragment {
		tags = append(tags, tag)
	}
	// Simple numeric sort (not canonical FIX order)
	for i := 0; i < len(tags); i++ {
		for j := i + 1; j < len(tags); j++ {
			if tags[j] < tags[i] {
				tags[i], tags[j] = tags[j], tags[i]
			}
		}
	}
	return tags
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

var tagNames = map[int]string{
	8:   "BeginString",
	9:   "BodyLength",
	10:  "CheckSum",
	11:  "ClOrdID",
	16:  "EndSeqNo",
	34:  "MsgSeqNum",
	35:  "MsgType",
	36:  "NewSeqNo",
	40:  "OrdType",
	43:  "PossDupFlag",
	44:  "Price",
	45:  "RefSeqNum",
	49:  "SenderCompID",
	50:  "SenderSubID",
	52:  "SendingTime",
	54:  "Side",
	55:  "Symbol",
	56:  "TargetCompID",
	57:  "TargetSubID",
	58:  "Text",
	108: "HeartBtInt",
	112: "TestReqID",
	115: "OnBehalfOf",
	122: "OrigSendTime",
	128: "DeliverTo",
	141: "ResetSeqNumFlag",
	143: "XmlData",
	145: "SecDataLen",
	212: "XmlDataLen",
	372: "RefMsgType",
	373: "SessionRejectReason",
}

func tagName(tag int) string {
	if name, ok := tagNames[tag]; ok {
		return name
	}
	return "Unknown"
}
