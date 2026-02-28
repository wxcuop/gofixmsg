package codec_test

import (
"strings"
"testing"

"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
"github.com/wxcuop/pyfixmsg_plus/fixmsg"
"github.com/wxcuop/pyfixmsg_plus/fixmsg/codec"
"github.com/wxcuop/pyfixmsg_plus/fixmsg/spec"
)

var minimalXML = []byte(`
<fix major="4" minor="4" type="FIX">
  <messages>
    <message name="NewOrderSingle" msgtype="D">
      <field name="ClOrdID"/>
      <field name="Symbol"/>
      <group name="NoAllocs">
        <field name="AllocAccount"/>
        <field name="AllocShares"/>
      </group>
    </message>
  </messages>
  <components></components>
  <fields>
    <field number="11" name="ClOrdID" type="STRING"/>
    <field number="55" name="Symbol" type="STRING"/>
    <field number="78" name="NoAllocs" type="NUMINGROUP"/>
    <field number="79" name="AllocAccount" type="STRING"/>
    <field number="80" name="AllocShares" type="QTY"/>
  </fields>
</fix>`)

func loadSpec(t *testing.T) *spec.FixSpec {
t.Helper()
s, err := spec.LoadBytes(minimalXML)
require.NoError(t, err)
return s
}

func TestCodec_ParseSimple(t *testing.T) {
c := codec.New(nil)
wire := "8=FIX.4.4\x019=20\x0135=D\x0149=SENDER\x0156=TARGET\x0110=123\x01"
m, err := c.Parse([]byte(wire))
require.NoError(t, err)

v, ok := m.Get(35)
assert.True(t, ok)
assert.Equal(t, "D", v)

v, ok = m.Get(49)
assert.True(t, ok)
assert.Equal(t, "SENDER", v)
}

func TestCodec_ParseRepeatingGroups(t *testing.T) {
s := loadSpec(t)
c := codec.New(s)
wire := "8=FIX.4.4\x0135=D\x0111=order1\x0155=AAPL\x0178=2\x0179=Acc1\x0180=100\x0179=Acc2\x0180=200\x0110=000\x01"
m, err := c.Parse([]byte(wire))
require.NoError(t, err)

rg, ok := m.GetGroup(78)
require.True(t, ok, "group tag 78 must be present")
assert.Equal(t, 2, rg.Len())

acc1, _ := rg.At(0).Get(79)
acc2, _ := rg.At(1).Get(79)
assert.Equal(t, "Acc1", acc1)
assert.Equal(t, "Acc2", acc2)
}

func TestCodec_SerialiseSimple(t *testing.T) {
c := codec.New(nil)
m := fixmsg.NewFixMessageFromMap(map[int]string{
8:  "FIX.4.4",
35: "D",
49: "SENDER",
56: "TARGET",
34: "1",
})
m.SetLenAndChecksum()

wire, err := c.Serialise(m)
require.NoError(t, err)
s := string(wire)
assert.True(t, strings.Contains(s, "8=FIX.4.4\x01"))
assert.True(t, strings.Contains(s, "35=D\x01"))
assert.True(t, strings.HasSuffix(strings.TrimRight(s, "\x01"), "10="+m.Checksum()))
}

func TestCodec_SerialiseWithGroup(t *testing.T) {
s := loadSpec(t)
c := codec.New(s)

m := fixmsg.NewFixMessageFromMap(map[int]string{
8:  "FIX.4.4",
35: "D",
11: "order1",
55: "AAPL",
})
rg := fixmsg.NewRepeatingGroup(78)
mb1 := rg.Add()
mb1.Set(79, "Acc1")
mb1.Set(80, "100")
mb2 := rg.Add()
mb2.Set(79, "Acc2")
mb2.Set(80, "200")
m.SetGroup(78, rg)
m.Set(fixmsg.TagBodyLength, "0")
m.Set(fixmsg.TagCheckSum, "000")

wire, err := c.Serialise(m)
require.NoError(t, err)
ws := string(wire)
assert.Contains(t, ws, "78=2\x01")
assert.Contains(t, ws, "79=Acc1\x01")
assert.Contains(t, ws, "79=Acc2\x01")
}

func TestCodec_RoundTrip_WithGroups(t *testing.T) {
s := loadSpec(t)
c := codec.New(s)

original := fixmsg.NewFixMessageFromMap(map[int]string{
8:  "FIX.4.4",
35: "D",
11: "order1",
55: "AAPL",
})
rg := fixmsg.NewRepeatingGroup(78)
mb1 := rg.Add()
mb1.Set(79, "Acc1")
mb1.Set(80, "100")
original.SetGroup(78, rg)
original.SetLenAndChecksum()

wire, err := c.Serialise(original)
require.NoError(t, err)

parsed, err := c.Parse(wire)
require.NoError(t, err)

for _, tag := range []int{8, 35, 11, 55} {
ov, _ := original.Get(tag)
pv, ok := parsed.Get(tag)
assert.True(t, ok, "tag %d missing after round-trip", tag)
assert.Equal(t, ov, pv, "tag %d mismatch after round-trip", tag)
}

prg, ok := parsed.GetGroup(78)
require.True(t, ok)
assert.Equal(t, 1, prg.Len())
acc, _ := prg.At(0).Get(79)
assert.Equal(t, "Acc1", acc)
}

func TestCodec_Parse_MalformedTag(t *testing.T) {
	// Codec is lenient (matches Python stringfix.py): non-integer tags are silently skipped.
	c := codec.New(nil)
	m, err := c.Parse([]byte("8=FIX.4.4\x01NOTANUMBER=D\x0110=000\x01"))
	require.NoError(t, err)
	v, ok := m.Get(8)
	assert.True(t, ok)
	assert.Equal(t, "FIX.4.4", v)
}

func TestCodec_Parse_Empty(t *testing.T) {
	// Empty input produces an empty FixMessage without error.
	c := codec.New(nil)
	m, err := c.Parse([]byte(""))
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.Empty(t, m.AllTags())
}
