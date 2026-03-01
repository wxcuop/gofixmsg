package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wxcuop/gofixmsg/fixmsg"
	"github.com/wxcuop/gofixmsg/fixmsg/spec"
)

func TestValidateMessage_MandatoryFields(t *testing.T) {
	// Create a valid message
	msg := fixmsg.NewFixMessage()
	msg.Set(8, "FIX.4.4")
	msg.Set(9, "10")
	msg.Set(35, "D")
	msg.Set(49, "SENDER")
	msg.Set(56, "TARGET")
	msg.Set(34, "1")
	msg.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))
	msg.Set(10, "123")

	err := ValidateMessage(msg, nil)
	assert.NoError(t, err, "expected valid message to pass")

	// Test missing mandatory tag
	msgMissing := fixmsg.NewFixMessage()
	msgMissing.Set(8, "FIX.4.4")
	msgMissing.Set(9, "10")
	msgMissing.Set(35, "D")
	msgMissing.Set(49, "SENDER")
	// missing 56
	msgMissing.Set(34, "1")
	msgMissing.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))
	msgMissing.Set(10, "123")

	err = ValidateMessage(msgMissing, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing mandatory tag 56")
}

func TestValidateMessage_DictionaryHardening(t *testing.T) {
	s := &spec.FixSpec{
		Tags: map[int]*spec.FixTag{
			11:  {Number: 11, Name: "ClOrdID", Type: "STRING"},
			21:  {Number: 21, Name: "HandlInst", Type: "CHAR", Values: map[string]string{"1": "AUTO", "2": "MANUAL"}},
			38:  {Number: 38, Name: "OrderQty", Type: "QTY"},
			40:  {Number: 40, Name: "OrdType", Type: "CHAR", Values: map[string]string{"1": "MARKET", "2": "LIMIT"}},
			44:  {Number: 44, Name: "Price", Type: "PRICE"},
			54:  {Number: 54, Name: "Side", Type: "CHAR", Values: map[string]string{"1": "BUY", "2": "SELL"}},
			55:  {Number: 55, Name: "Symbol", Type: "STRING"},
			60:  {Number: 60, Name: "TransTime", Type: "UTCTIMESTAMP"},
		},
		MessagesByType: map[string]*spec.MessageSpec{
			"D": {
				Name:    "NewOrderSingle",
				MsgType: "D",
				RequiredTags: map[int]struct{}{
					11: {}, 21: {}, 40: {}, 54: {}, 55: {}, 60: {},
				},
			},
		},
	}

	msg := fixmsg.NewFixMessage()
	// Header
	msg.Set(8, "FIX.4.4")
	msg.Set(9, "100")
	msg.Set(35, "D")
	msg.Set(49, "S")
	msg.Set(56, "T")
	msg.Set(34, "1")
	msg.Set(52, "20260228-12:00:00.000")
	// Body
	msg.Set(11, "ORD1")
	msg.Set(21, "1")
	msg.Set(40, "2")
	msg.Set(44, "10.50")
	msg.Set(54, "1")
	msg.Set(55, "AAPL")
	msg.Set(60, "20260228-12:00:00.000")
	// Trailer
	msg.Set(10, "123")

	err := ValidateMessage(msg, s)
	assert.NoError(t, err)

	// 1. Test missing required field per message type
	msgMissing := fixmsg.NewFixMessage()
	for k, v := range msg.FixFragment { if k != 11 { msgMissing.Set(k, v.(string)) } }
	err = ValidateMessage(msgMissing, s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required tag 11 for MsgType D")

	// 2. Test invalid enum value
	msgInvalidEnum := fixmsg.NewFixMessage()
	for k, v := range msg.FixFragment { msgInvalidEnum.Set(k, v.(string)) }
	msgInvalidEnum.Set(54, "3") // Invalid side
	err = ValidateMessage(msgInvalidEnum, s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid enum value \"3\" for tag 54 (Side)")

	// 3. Test invalid data type
	msgInvalidType := fixmsg.NewFixMessage()
	for k, v := range msg.FixFragment { msgInvalidType.Set(k, v.(string)) }
	msgInvalidType.Set(38, "abc") // Invalid QTY
	err = ValidateMessage(msgInvalidType, s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value for tag 38 (OrderQty): expected float")
}

func TestValidateMessage_RepeatingGroups(t *testing.T) {
	s := &spec.FixSpec{
		Tags: map[int]*spec.FixTag{
			453: {Number: 453, Name: "NoPartyIDs", Type: "NUMINGROUP"},
			448: {Number: 448, Name: "PartyID", Type: "STRING"},
			447: {Number: 447, Name: "PartyIDSource", Type: "CHAR"},
			452: {Number: 452, Name: "PartyRole", Type: "INT"},
		},
		MessagesByType: map[string]*spec.MessageSpec{
			"D": {
				Name:    "NewOrderSingle",
				MsgType: "D",
				Groups: map[int]*spec.GroupSpec{
					453: {
						NumberTag: 453,
						RequiredTags: map[int]struct{}{448: {}, 447: {}, 452: {}},
					},
				},
			},
		},
	}

	msg := fixmsg.NewFixMessage()
	msg.Set(8, "FIX.4.4"); msg.Set(9, "10"); msg.Set(35, "D"); msg.Set(49, "S"); msg.Set(56, "T"); msg.Set(34, "1"); msg.Set(52, "20260228-12:00:00.000"); msg.Set(10, "123")
	
	group := fixmsg.NewRepeatingGroup(453)
	m1 := group.Add()
	m1.Set(448, "ID1")
	m1.Set(447, "D")
	m1.Set(452, "1")
	msg.SetGroup(453, group)

	err := ValidateMessage(msg, s)
	assert.NoError(t, err)

	// Missing field in group member
	delete(m1, 448)
	err = ValidateMessage(msg, s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "group 453: member 0: missing required tag 448")
}
