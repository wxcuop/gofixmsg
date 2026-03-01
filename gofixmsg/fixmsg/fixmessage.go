package fixmsg

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Codec is the interface that bridges [FixMessage] to its wire-format
// implementation in [fixmsg/codec]. Defining it here lets codec import fixmsg
// without creating a circular dependency.
type Codec interface {
	// Serialise converts msg to its FIX wire-format bytes.
	// The caller should call [FixMessage.SetLenAndChecksum] first.
	Serialise(msg *FixMessage) ([]byte, error)
	// Parse creates a new FixMessage from FIX wire-format bytes.
	Parse(buf []byte) (*FixMessage, error)
}

// FixMessage is the top-level FIX message type.
//
// It embeds [FixFragment] for tag storage and carries a [Codec] reference for
// wire-format operations. When Codec is nil a simple built-in serialiser is
// used that does not support repeating groups.
type FixMessage struct {
	FixFragment

	// Codec handles serialise/parse; nil uses the built-in default.
	Codec Codec

	// Time is when the message was created or received. Defaults to UTC now.
	Time time.Time
	// Direction: 0=inbound, 1=outbound, -1=unknown.
	Direction int
	// RawMessage holds the original wire bytes when parsed via LoadFix.
	RawMessage []byte
}

// NewFixMessage returns an empty FixMessage with Time set to now (UTC).
func NewFixMessage() *FixMessage {
	return &FixMessage{
		FixFragment: make(FixFragment),
		Time:        time.Now().UTC(),
		Direction:   -1,
	}
}

// NewFixMessageFromMap returns a FixMessage pre-populated from fields.
func NewFixMessageFromMap(fields map[int]string) *FixMessage {
	m := NewFixMessage()
	for k, v := range fields {
		m.Set(k, v)
	}
	return m
}

// ToWire serialises the message to FIX wire-format bytes.
// SetLenAndChecksum is called internally before serialisation.
func (m *FixMessage) ToWire() ([]byte, error) {
	m.SetLenAndChecksum()
	if m.Codec != nil {
		return m.Codec.Serialise(m)
	}
	return defaultSerialise(m)
}

// LoadFix parses buf into m, replacing any existing tags.
// Uses m.Codec when set, otherwise the built-in default (no repeating groups).
func (m *FixMessage) LoadFix(buf []byte) error {
	var (
		parsed *FixMessage
		err    error
	)
	if m.Codec != nil {
		parsed, err = m.Codec.Parse(buf)
	} else {
		parsed, err = defaultParse(buf)
	}
	if err != nil {
		return fmt.Errorf("fixmsg: LoadFix: %w", err)
	}
	for k, v := range parsed.FixFragment {
		m.FixFragment[k] = v
	}
	m.RawMessage = buf
	return nil
}

// SetLenAndChecksum computes BodyLength (tag 9) and CheckSum (tag 10) and
// stores them in the message. It must be called before ToWire.
//
// Algorithm (FIX standard):
//  1. Build body bytes = all fields except tags 8, 9, 10 in canonical order.
//  2. BodyLength  = len(body bytes).
//  3. prefix      = "8=<BeginString>\x019=<BodyLength>\x01"
//  4. CheckSum    = (sum of all bytes in prefix + body) mod 256, zero-padded to 3 digits.
func (m *FixMessage) SetLenAndChecksum() {
	body := buildBody(m)

	beginStr, _ := m.Get(TagBeginString)
	bodyLen := len(body)

	prefix := fmt.Sprintf("%d=%s\x01%d=%d\x01", TagBeginString, beginStr, TagBodyLength, bodyLen)
	chk := 0
	for _, b := range []byte(prefix) {
		chk += int(b)
	}
	for _, b := range body {
		chk += int(b)
	}
	m.Set(TagBodyLength, strconv.Itoa(bodyLen))
	m.Set(TagCheckSum, fmt.Sprintf("%03d", chk%256))
}

// Length returns the body byte count (tag 9). Calls SetLenAndChecksum if needed.
func (m *FixMessage) Length() int {
	if v, ok := m.Get(TagBodyLength); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return len(buildBody(m))
}

// Checksum returns the three-digit checksum string (tag 10).
// Calls SetLenAndChecksum if tag 10 is not already set.
func (m *FixMessage) Checksum() string {
	if v, ok := m.Get(TagCheckSum); ok {
		return v
	}
	m.SetLenAndChecksum()
	v, _ := m.Get(TagCheckSum)
	return v
}

// TagExact returns true if the string value of tag equals value.
func (m *FixMessage) TagExact(tag int, value string, caseInsensitive bool) bool {
	mine, ok := m.Get(tag)
	if !ok {
		return false
	}
	if caseInsensitive {
		return strings.EqualFold(mine, value)
	}
	return mine == value
}

// TagContains returns true if the string value of tag contains value.
func (m *FixMessage) TagContains(tag int, value string, caseInsensitive bool) bool {
	mine, ok := m.Get(tag)
	if !ok {
		return false
	}
	if caseInsensitive {
		return strings.Contains(strings.ToLower(mine), strings.ToLower(value))
	}
	return strings.Contains(mine, value)
}

// SetOrDelete sets tag to value, or removes the tag if value is empty.
func (m *FixMessage) SetOrDelete(tag int, value string) {
	if value == "" {
		delete(m.FixFragment, tag)
	} else {
		m.Set(tag, value)
	}
}

// ---- built-in (no-spec) serialiser / parser ----

// defaultSerialise writes tags in canonical FIX order: header → body → trailer.
func defaultSerialise(m *FixMessage) ([]byte, error) {
	var buf strings.Builder
	for _, tag := range sortedTags(m.FixFragment) {
		if err := writeField(&buf, tag, m.FixFragment[tag]); err != nil {
			return nil, err
		}
	}
	return []byte(buf.String()), nil
}

func writeField(buf *strings.Builder, tag int, val any) error {
	switch v := val.(type) {
	case string:
		buf.WriteString(strconv.Itoa(tag))
		buf.WriteByte('=')
		buf.WriteString(v)
		buf.WriteByte('\x01')
	case *RepeatingGroup:
		buf.WriteString(strconv.Itoa(tag))
		buf.WriteByte('=')
		buf.WriteString(strconv.Itoa(v.Len()))
		buf.WriteByte('\x01')
		for _, member := range v.Members {
			for _, mt := range sortedTags(member) {
				if err := writeField(buf, mt, member[mt]); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("fixmsg: unsupported value type for tag %d: %T", tag, val)
	}
	return nil
}

// defaultParse parses a SOH-delimited FIX buffer without repeating group support.
func defaultParse(buf []byte) (*FixMessage, error) {
	m := NewFixMessage()
	if len(buf) == 0 {
		return m, nil
	}
	s := string(buf)
	for _, field := range strings.Split(s, "\x01") {
		if field == "" {
			continue
		}
		idx := strings.IndexByte(field, '=')
		if idx < 0 {
			return nil, fmt.Errorf("fixmsg: malformed field (no '='): %q", field)
		}
		tag, err := strconv.Atoi(field[:idx])
		if err != nil {
			// Non-integer tag; skip (spec-less parser limitation).
			continue
		}
		m.Set(tag, field[idx+1:])
	}
	return m, nil
}

// buildBody returns wire bytes for all fields EXCEPT tags 8, 9, 10 in
// canonical FIX order. Used for BodyLength and CheckSum calculation.
func buildBody(m *FixMessage) []byte {
	var buf strings.Builder
	for _, tag := range sortedTags(m.FixFragment) {
		if tag == TagBeginString || tag == TagBodyLength || tag == TagCheckSum {
			continue
		}
		_ = writeField(&buf, tag, m.FixFragment[tag])
	}
	return []byte(buf.String())
}

// sortedTags returns the tag numbers of f sorted in canonical FIX order.
func sortedTags(f FixFragment) []int {
	tags := make([]int, 0, len(f))
	for t := range f {
		tags = append(tags, t)
	}
	sort.Slice(tags, func(i, j int) bool {
		return tagSortKey(tags[i]) < tagSortKey(tags[j])
	})
	return tags
}
