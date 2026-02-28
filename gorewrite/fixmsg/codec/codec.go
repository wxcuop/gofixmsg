// Package codec implements the FIX wire-format codec.
//
// Use [New] to create a [Codec] with or without a spec.
// Without a spec, repeating groups are not parsed.
package codec

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg/spec"
)

const (
	// SOH is the standard FIX field separator (ASCII 0x01).
	SOH = '\x01'
	// Delimiter separates tag from value within a field.
	Delimiter = '='
)

// Codec parses and serialises FIX messages.
// It implements [fixmsg.Codec] and can be stored in [fixmsg.FixMessage.Codec].
type Codec struct {
	Spec      *spec.FixSpec
	Separator byte
	noGroups  bool
}

// New returns a Codec using the provided spec (may be nil).
func New(s *spec.FixSpec) *Codec {
	return &Codec{Spec: s, Separator: SOH}
}

// NewNoGroups returns a Codec that never parses repeating groups.
func NewNoGroups() *Codec {
	return &Codec{Separator: SOH, noGroups: true}
}

// WithSeparator returns a copy of c that uses sep instead of SOH.
func (c *Codec) WithSeparator(sep byte) *Codec {
	cp := *c
	cp.Separator = sep
	return &cp
}

// Parse creates a [fixmsg.FixMessage] from FIX wire-format bytes.
func (c *Codec) Parse(buf []byte) (*fixmsg.FixMessage, error) {
	sep := c.Separator
	if sep == 0 {
		sep = SOH
	}
	pairs, err := tokenise(buf, Delimiter, sep)
	if err != nil {
		return nil, fmt.Errorf("codec: parse: %w", err)
	}
	if len(pairs) == 0 {
		return fixmsg.NewFixMessage(), nil
	}

	var msgSpec *spec.MessageSpec
	if !c.noGroups && c.Spec != nil {
		limit := len(pairs)
		if limit > 6 {
			limit = 6
		}
		for _, p := range pairs[:limit] {
			if p[0] == "35" {
				msgSpec = c.Spec.MessageByType(p[1])
				break
			}
		}
	}

	msg := fixmsg.NewFixMessage()
	if msgSpec == nil || c.noGroups {
		for _, p := range pairs {
			tag, err := strconv.Atoi(p[0])
			if err != nil {
				continue
			}
			msg.Set(tag, p[1])
		}
		return msg, nil
	}

	i := 0
	for i < len(pairs) {
		tag, err := strconv.Atoi(pairs[i][0])
		if err != nil {
			i++
			continue
		}
		value := pairs[i][1]
		if grpSpec, ok := msgSpec.Groups[tag]; ok {
			count, _ := strconv.Atoi(value)
			rg := fixmsg.NewRepeatingGroup(tag)
			rg.FirstTag = grpSpec.FirstTag
			i++
			i, _ = parseGroup(pairs, i, rg, grpSpec, count)
			msg.SetGroup(tag, rg)
		} else {
			msg.Set(tag, value)
			i++
		}
	}
	return msg, nil
}

func parseGroup(pairs [][2]string, offset int, rg *fixmsg.RepeatingGroup, grpSpec *spec.GroupSpec, count int) (int, int) {
	if count == 0 {
		return offset, 0
	}
	firstTag := grpSpec.FirstTag
	var current fixmsg.FixFragment
	for offset < len(pairs) && rg.Len() < count {
		tag, err := strconv.Atoi(pairs[offset][0])
		if err != nil {
			break
		}
		value := pairs[offset][1]
		if tag == firstTag {
			if current != nil {
				rg.Members = append(rg.Members, current)
			}
			current = make(fixmsg.FixFragment)
			current.Set(tag, value)
			offset++
			continue
		}
		if current == nil {
			break
		}
		if _, inGroup := grpSpec.Tags[tag]; !inGroup {
			break
		}
		if subSpec, ok := grpSpec.NestedGroups[tag]; ok {
			subCount, _ := strconv.Atoi(value)
			subRG := fixmsg.NewRepeatingGroup(tag)
			subRG.FirstTag = subSpec.FirstTag
			offset++
			offset, _ = parseGroup(pairs, offset, subRG, subSpec, subCount)
			current.SetGroup(tag, subRG)
		} else {
			current.Set(tag, value)
			offset++
		}
	}
	if current != nil {
		rg.Members = append(rg.Members, current)
	}
	return offset, rg.Len()
}

// Serialise converts msg to FIX wire-format bytes.
func (c *Codec) Serialise(msg *fixmsg.FixMessage) ([]byte, error) {
	sep := c.Separator
	if sep == 0 {
		sep = SOH
	}
	var buf strings.Builder
	for _, tag := range sortedTagsOf(msg.FixFragment) {
		if err := writeVal(&buf, tag, msg.FixFragment[tag], sep); err != nil {
			return nil, fmt.Errorf("codec: serialise tag %d: %w", tag, err)
		}
	}
	return []byte(buf.String()), nil
}

func writeVal(buf *strings.Builder, tag int, val any, sep byte) error {
	switch v := val.(type) {
	case string:
		buf.WriteString(strconv.Itoa(tag))
		buf.WriteByte(Delimiter)
		buf.WriteString(v)
		buf.WriteByte(sep)
	case *fixmsg.RepeatingGroup:
		buf.WriteString(strconv.Itoa(tag))
		buf.WriteByte(Delimiter)
		buf.WriteString(strconv.Itoa(v.Len()))
		buf.WriteByte(sep)
		for _, member := range v.Members {
			for _, mt := range sortedTagsOf(member) {
				if err := writeVal(buf, mt, member[mt], sep); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("unsupported value type %T for tag %d", val, tag)
	}
	return nil
}

func tokenise(buf []byte, delimiter, separator byte) ([][2]string, error) {
	s := string(buf)
	fields := strings.Split(s, string(separator))
	pairs := make([][2]string, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}
		idx := strings.IndexByte(field, delimiter)
		if idx < 0 {
			return nil, fmt.Errorf("malformed field (no %q): %q", delimiter, field)
		}
		pairs = append(pairs, [2]string{field[:idx], field[idx+1:]})
	}
	return pairs, nil
}

func sortedTagsOf(f fixmsg.FixFragment) []int {
	tags := f.AllTags()
	sort.Slice(tags, func(i, j int) bool {
		return tagKey(tags[i]) < tagKey(tags[j])
	})
	return tags
}

func tagKey(t int) int {
	trailerOrder := map[int]int{93: 1<<30 - 3, 89: 1<<30 - 2, 10: 1<<30 - 1}
	if k, ok := trailerOrder[t]; ok {
		return k
	}
	if k, ok := fixmsg.HeaderSortMap[t]; ok {
		return k
	}
	return len(fixmsg.HeaderTags) + t
}
