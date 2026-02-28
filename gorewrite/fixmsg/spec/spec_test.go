package spec_test

import (
"testing"

"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
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
    <message name="Heartbeat" msgtype="0">
      <field name="TestReqID"/>
    </message>
  </messages>
  <components>
  </components>
  <fields>
    <field number="1" name="Account" type="STRING"/>
    <field number="11" name="ClOrdID" type="STRING"/>
    <field number="55" name="Symbol" type="STRING"/>
    <field number="78" name="NoAllocs" type="NUMINGROUP"/>
    <field number="79" name="AllocAccount" type="STRING"/>
    <field number="80" name="AllocShares" type="QTY"/>
    <field number="112" name="TestReqID" type="STRING"/>
  </fields>
</fix>`)

func TestLoadBytes_BasicSpec(t *testing.T) {
s, err := spec.LoadBytes(minimalXML)
require.NoError(t, err)
require.NotNil(t, s)

assert.Equal(t, "FIX.4.4", s.Version)
}

func TestLoadBytes_Tags(t *testing.T) {
s, err := spec.LoadBytes(minimalXML)
require.NoError(t, err)

ft := s.TagByNumber(55)
require.NotNil(t, ft)
assert.Equal(t, "Symbol", ft.Name)
assert.Equal(t, "STRING", ft.Type)
}

func TestLoadBytes_MessageByType(t *testing.T) {
s, err := spec.LoadBytes(minimalXML)
require.NoError(t, err)

ms := s.MessageByType("D")
require.NotNil(t, ms)
assert.Equal(t, "NewOrderSingle", ms.Name)
}

func TestLoadBytes_MessageByType_Missing(t *testing.T) {
s, err := spec.LoadBytes(minimalXML)
require.NoError(t, err)
assert.Nil(t, s.MessageByType("ZZ"))
}

func TestLoadBytes_GroupSpec(t *testing.T) {
s, err := spec.LoadBytes(minimalXML)
require.NoError(t, err)

ms := s.MessageByType("D")
require.NotNil(t, ms)

gs, ok := ms.Groups[78]
require.True(t, ok, "NoAllocs group (tag 78) must be present")
assert.Equal(t, 78, gs.NumberTag)
assert.Equal(t, 79, gs.FirstTag, "AllocAccount (79) must be first tag in group")
assert.Contains(t, gs.Tags, 79)
assert.Contains(t, gs.Tags, 80)
}

func TestLoadBytes_TagNumber(t *testing.T) {
s, err := spec.LoadBytes(minimalXML)
require.NoError(t, err)

n, ok := s.TagNumber("Symbol")
assert.True(t, ok)
assert.Equal(t, 55, n)

_, ok = s.TagNumber("UnknownTag")
assert.False(t, ok)
}

func TestLoadBytes_InvalidXML(t *testing.T) {
_, err := spec.LoadBytes([]byte("<broken"))
assert.Error(t, err)
}
