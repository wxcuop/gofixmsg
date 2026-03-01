package engine

import (
	"fmt"
	"strconv"
	"time"

	"github.com/wxcuop/gofixmsg/fixmsg"
	"github.com/wxcuop/gofixmsg/fixmsg/spec"
)

// ValidateMessage checks if a FIX message is structurally valid against standard rules
// and optionally against a Data Dictionary (FixSpec) if provided.
func ValidateMessage(m *fixmsg.FixMessage, s *spec.FixSpec) error {
	// 1. Basic mandatory header/trailer tags (always required)
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

	// 2. Dictionary-based validation
	if s != nil {
		msgType, _ := m.Get(fixmsg.TagMsgType)
		msgSpec := s.MessageByType(msgType)
		if msgSpec == nil {
			return fmt.Errorf("unknown MsgType %q in dictionary", msgType)
		}

		// Validate required fields for this message type
		for tag := range msgSpec.RequiredTags {
			if !m.Contains(tag) {
				return fmt.Errorf("missing required tag %d for MsgType %s (%s)", tag, msgType, msgSpec.Name)
			}
		}

		// Validate all tags present in the message
		for tagNum, valAny := range m.FixFragment {
			valStr, ok := valAny.(string)
			if !ok {
				// Deep validation for repeating groups
				if group, ok := valAny.(*fixmsg.RepeatingGroup); ok {
					if err := validateRepeatingGroup(group, msgSpec.Groups[tagNum], s); err != nil {
						return fmt.Errorf("group %d: %w", tagNum, err)
					}
				}
				continue
			}

			tagDef := s.TagByNumber(tagNum)
			if tagDef == nil {
				// Optionally reject unknown tags? Usually allowed in FIX unless strict.
				continue
			}

			// Validate data type
			if err := validateDataType(tagDef.Type, valStr); err != nil {
				return fmt.Errorf("invalid value for tag %d (%s): %w", tagNum, tagDef.Name, err)
			}

			// Validate enum values if defined
			if len(tagDef.Values) > 0 {
				if _, ok := tagDef.Values[valStr]; !ok {
					return fmt.Errorf("invalid enum value %q for tag %d (%s)", valStr, tagNum, tagDef.Name)
				}
			}
		}
	}

	return nil
}

func validateRepeatingGroup(g *fixmsg.RepeatingGroup, gSpec *spec.GroupSpec, s *spec.FixSpec) error {
	if gSpec == nil {
		// Group not in spec
		return nil
	}

	for i, member := range g.Members {
		// Check required fields in group member
		for tag := range gSpec.RequiredTags {
			if !member.Contains(tag) {
				return fmt.Errorf("member %d: missing required tag %d", i, tag)
			}
		}

		// Validate fields in member
		for tagNum, valAny := range member {
			valStr, ok := valAny.(string)
			if !ok {
				if subGroup, ok := valAny.(*fixmsg.RepeatingGroup); ok {
					if err := validateRepeatingGroup(subGroup, gSpec.NestedGroups[tagNum], s); err != nil {
						return fmt.Errorf("member %d: sub-group %d: %w", i, tagNum, err)
					}
				}
				continue
			}

			tagDef := s.TagByNumber(tagNum)
			if tagDef == nil {
				continue
			}

			if err := validateDataType(tagDef.Type, valStr); err != nil {
				return fmt.Errorf("member %d: invalid value for tag %d (%s): %w", i, tagNum, tagDef.Name, err)
			}

			if len(tagDef.Values) > 0 {
				if _, ok := tagDef.Values[valStr]; !ok {
					return fmt.Errorf("member %d: invalid enum value %q for tag %d (%s)", i, valStr, tagNum, tagDef.Name)
				}
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
		// Format: YYYYMMDD-HH:MM:SS or YYYYMMDD-HH:MM:SS.sss or YYYYMMDD-HH:MM:SS.ssssss
		if len(value) < 17 {
			return fmt.Errorf("UTCTimestamp too short")
		}
		layout := "20060102-15:04:05"
		if len(value) == 21 {
			layout = "20060102-15:04:05.000"
		} else if len(value) == 24 {
			layout = "20060102-15:04:05.000000"
		}
		if _, err := time.Parse(layout, value); err != nil {
			return fmt.Errorf("invalid UTCTimestamp: %w", err)
		}
	case "UTCDATEONLY":
		if _, err := time.Parse("20060102", value); err != nil {
			return fmt.Errorf("invalid UTCDateOnly: %w", err)
		}
	case "UTCTIMEONLY":
		layout := "15:04:05"
		if len(value) == 12 {
			layout = "15:04:05.000"
		}
		if _, err := time.Parse(layout, value); err != nil {
			return fmt.Errorf("invalid UTCTimeOnly: %w", err)
		}
	case "BOOLEAN":
		if value != "Y" && value != "N" {
			return fmt.Errorf("invalid boolean %q (expected Y/N)", value)
		}
	}
	return nil
}
