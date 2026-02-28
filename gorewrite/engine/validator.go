package engine

import (
	"fmt"
	"strconv"
	"time"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg/spec"
)

// ValidateMessage checks if a FIX message is structurally valid against standard rules
// and optionally against a Data Dictionary (FixSpec) if provided.
func ValidateMessage(m *fixmsg.FixMessage, s *spec.FixSpec) error {
	// Check mandatory header and trailer fields
	mandatoryTags := []int{
		fixmsg.TagBeginString,  // 8
		fixmsg.TagBodyLength,   // 9
		fixmsg.TagMsgType,      // 35
		fixmsg.TagSenderCompID, // 49
		fixmsg.TagTargetCompID, // 56
		fixmsg.TagMsgSeqNum,    // 34
		fixmsg.TagSendingTime,  // 52
		fixmsg.TagCheckSum,     // 10
	}

	for _, tag := range mandatoryTags {
		if !m.Contains(tag) {
			return fmt.Errorf("missing mandatory tag %d", tag)
		}
	}

	// Validate data types if a dictionary spec is provided
	if s != nil {
		for tagNum, valAny := range m.FixFragment {
			valStr, ok := valAny.(string)
			if !ok {
				// Skip repeating groups for this basic validation
				continue
			}

			tagSpec := s.TagByNumber(tagNum)
			if tagSpec == nil {
				// Unknown tag according to dictionary
				continue
			}

			if err := validateDataType(tagSpec.Type, valStr); err != nil {
				return fmt.Errorf("invalid value for tag %d (%s): %w", tagNum, tagSpec.Name, err)
			}
		}
	}

	return nil
}

func validateDataType(dataType string, value string) error {
	switch dataType {
	case "INT", "LENGTH", "NUMINGROUP", "SEQNUM":
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("expected integer, got %q", value)
		}
	case "FLOAT", "QTY", "PRICE", "PRICEOFFSET", "AMT", "PERCENTAGE":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("expected float, got %q", value)
		}
	case "UTCTIMESTAMP":
		// Format: YYYYMMDD-HH:MM:SS or YYYYMMDD-HH:MM:SS.sss
		if len(value) != 17 && len(value) != 21 {
			return fmt.Errorf("invalid UTCTimestamp format/length")
		}
		layout := "20060102-15:04:05"
		if len(value) == 21 {
			layout = "20060102-15:04:05.000"
		}
		if _, err := time.Parse(layout, value); err != nil {
			return fmt.Errorf("invalid UTCTimestamp: %w", err)
		}
	case "UTCDATEONLY":
		if _, err := time.Parse("20060102", value); err != nil {
			return fmt.Errorf("invalid UTCDateOnly: %w", err)
		}
	case "UTCTIMEONLY":
		// Format: HH:MM:SS or HH:MM:SS.sss
		layout := "15:04:05"
		if len(value) == 12 {
			layout = "15:04:05.000"
		}
		if _, err := time.Parse(layout, value); err != nil {
			return fmt.Errorf("invalid UTCTimeOnly: %w", err)
		}
	}
	return nil
}
