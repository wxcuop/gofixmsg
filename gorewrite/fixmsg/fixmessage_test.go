package fixmsg_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
)

// ---- FixFragment tests ----

func TestFixFragment_GetSet(t *testing.T) {
	f := make(fixmsg.FixFragment)
	f.Set(35, "D")
	v, ok := f.Get(35)
	assert.True(t, ok)
	assert.Equal(t, "D", v)
}

func TestFixFragment_GetAbsent(t *testing.T) {
	f := make(fixmsg.FixFragment)
	v, ok := f.Get(99)
	assert.False(t, ok)
	assert.Equal(t, "", v)
}

func TestFixFragment_MustGetPanics(t *testing.T) {
	f := make(fixmsg.FixFragment)
	assert.Panics(t, func() { f.MustGet(99) })
}

func TestFixFragment_Contains(t *testing.T) {
	f := make(fixmsg.FixFragment)
	f.Set(49, "SENDER")
	assert.True(t, f.Contains(49))
	assert.False(t, f.Contains(56))
}

func TestFixFragment_GetSetGroup(t *testing.T) {
	f := make(fixmsg.FixFragment)
	rg := fixmsg.NewRepeatingGroup(78)
	f.SetGroup(78, rg)
	got, ok := f.GetGroup(78)
	assert.True(t, ok)
	assert.Same(t, rg, got)
}

func TestFixFragment_FindAll_ScalarTag(t *testing.T) {
	f := make(fixmsg.FixFragment)
	f.Set(49, "SENDER")
	paths := f.FindAll(49)
	require.Len(t, paths, 1)
	assert.Equal(t, []any{49}, paths[0])
}

func TestFixFragment_FindAll_InGroup(t *testing.T) {
	f := make(fixmsg.FixFragment)
	rg := fixmsg.NewRepeatingGroup(78)
	m := rg.Add()
	m.Set(79, "AccountA")
	f.SetGroup(78, rg)

	paths := f.FindAll(79)
	require.Len(t, paths, 1)
	assert.Equal(t, []any{78, 0, 79}, paths[0])
}

func TestFixFragment_Anywhere(t *testing.T) {
	f := make(fixmsg.FixFragment)
	rg := fixmsg.NewRepeatingGroup(78)
	m := rg.Add()
	m.Set(79, "AccountA")
	f.SetGroup(78, rg)

	assert.True(t, f.Anywhere(79))
	assert.False(t, f.Anywhere(999))
}

func TestFixFragment_AllTagsDeep(t *testing.T) {
	f := make(fixmsg.FixFragment)
	f.Set(35, "D")
	rg := fixmsg.NewRepeatingGroup(78)
	m := rg.Add()
	m.Set(79, "Acc")
	f.SetGroup(78, rg)

	tags := f.AllTagsDeep()
	tagSet := make(map[int]struct{}, len(tags))
	for _, t := range tags {
		tagSet[t] = struct{}{}
	}
	assert.Contains(t, tagSet, 35)
	assert.Contains(t, tagSet, 78)
	assert.Contains(t, tagSet, 79)
}

// ---- RepeatingGroup tests ----

func TestRepeatingGroup_AddAndAt(t *testing.T) {
	rg := fixmsg.NewRepeatingGroup(78)
	assert.Equal(t, 0, rg.Len())

	m1 := rg.Add()
	m1.Set(79, "AccountA")
	m2 := rg.Add()
	m2.Set(79, "AccountB")

	assert.Equal(t, 2, rg.Len())
	v1, _ := rg.At(0).Get(79)
	v2, _ := rg.At(1).Get(79)
	assert.Equal(t, "AccountA", v1)
	assert.Equal(t, "AccountB", v2)
}

func TestRepeatingGroup_FindAll(t *testing.T) {
	rg := fixmsg.NewRepeatingGroup(78)
	m := rg.Add()
	m.Set(79, "Acc")
	paths := rg.FindAll(79)
	require.Len(t, paths, 1)
	assert.Equal(t, []any{0, 79}, paths[0])
}

// ---- FixMessage tests ----

func TestNewFixMessage(t *testing.T) {
	m := fixmsg.NewFixMessage()
	assert.NotNil(t, m)
	assert.False(t, m.Time.IsZero())
}

func TestNewFixMessageFromMap(t *testing.T) {
	m := fixmsg.NewFixMessageFromMap(map[int]string{
		8:  "FIX.4.4",
		35: "D",
		49: "SENDER",
	})
	v, ok := m.Get(35)
	assert.True(t, ok)
	assert.Equal(t, "D", v)
}

func TestFixMessage_SetLenAndChecksum(t *testing.T) {
	m := fixmsg.NewFixMessageFromMap(map[int]string{
		8:  "FIX.4.4",
		35: "0",
		49: "SENDER",
		56: "TARGET",
		34: "1",
		52: "20240101-00:00:00",
	})
	m.SetLenAndChecksum()

	lenVal, okLen := m.Get(fixmsg.TagBodyLength)
	chkVal, okChk := m.Get(fixmsg.TagCheckSum)
	assert.True(t, okLen)
	assert.True(t, okChk)
	assert.NotEmpty(t, lenVal)
	assert.Len(t, chkVal, 3, "checksum must be 3 digits")
}

func TestFixMessage_ChecksumIs3Digits(t *testing.T) {
	m := fixmsg.NewFixMessageFromMap(map[int]string{
		8: "FIX.4.4", 35: "0", 49: "A", 56: "B", 34: "1", 52: "20240101-00:00:00",
	})
	chk := m.Checksum()
	assert.Len(t, chk, 3)
	for _, c := range chk {
		assert.True(t, c >= '0' && c <= '9', "checksum must be digits")
	}
}

func TestFixMessage_ToWireAndLoadFix_RoundTrip(t *testing.T) {
	original := fixmsg.NewFixMessageFromMap(map[int]string{
		8:  "FIX.4.4",
		35: "D",
		49: "SENDER",
		56: "TARGET",
		34: "1",
	})

	wire, err := original.ToWire()
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(wire), "35=D\x01"))
	assert.True(t, strings.Contains(string(wire), "8=FIX.4.4\x01"))

	parsed := fixmsg.NewFixMessage()
	err = parsed.LoadFix(wire)
	require.NoError(t, err)

	for _, tag := range []int{8, 35, 49, 56, 34} {
		orig, _ := original.Get(tag)
		got, ok := parsed.Get(tag)
		assert.True(t, ok, "tag %d missing after round-trip", tag)
		assert.Equal(t, orig, got, "tag %d value mismatch", tag)
	}
}

func TestFixMessage_ToWire_HeaderFirst(t *testing.T) {
	// tag 8, 9, 35 must appear before body tags in wire output
	m := fixmsg.NewFixMessageFromMap(map[int]string{
		55: "AAPL", // body tag (Symbol)
		8:  "FIX.4.4",
		35: "D",
		49: "SENDER",
		56: "TARGET",
		34: "1",
	})
	wire, err := m.ToWire()
	require.NoError(t, err)
	s := string(wire)
	idx8 := strings.Index(s, "8=")
	idx35 := strings.Index(s, "35=")
	idx55 := strings.Index(s, "55=")
	assert.True(t, idx8 < idx35, "tag 8 must precede tag 35")
	assert.True(t, idx35 < idx55, "tag 35 must precede tag 55")
}

func TestFixMessage_ToWire_ChecksumLast(t *testing.T) {
	m := fixmsg.NewFixMessageFromMap(map[int]string{
		8: "FIX.4.4", 35: "D", 49: "SENDER", 56: "TARGET", 34: "1",
	})
	wire, err := m.ToWire()
	require.NoError(t, err)
	s := string(wire)
	// Strip trailing SOH before checking position.
	s = strings.TrimRight(s, "\x01")
	assert.True(t, strings.HasSuffix(s, "10="+m.Checksum()), "checksum must be last field")
}

func TestFixMessage_LoadFix_MalformedField(t *testing.T) {
	m := fixmsg.NewFixMessage()
	err := m.LoadFix([]byte("8=FIX.4.4\x01BADFIELD\x01"))
	assert.Error(t, err)
}

func TestFixMessage_TagExact(t *testing.T) {
	m := fixmsg.NewFixMessageFromMap(map[int]string{35: "D"})
	assert.True(t, m.TagExact(35, "D", false))
	assert.False(t, m.TagExact(35, "d", false))
	assert.True(t, m.TagExact(35, "d", true))
}

func TestFixMessage_SetOrDelete(t *testing.T) {
	m := fixmsg.NewFixMessageFromMap(map[int]string{35: "D"})
	m.SetOrDelete(35, "")
	assert.False(t, m.Contains(35))
	m.SetOrDelete(35, "0")
	assert.True(t, m.Contains(35))
}
