package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg/spec"
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

func TestValidateMessage_DataTypes(t *testing.T) {
	s := &spec.FixSpec{
		Tags: map[int]*spec.FixTag{
			11:  {Number: 11, Name: "ClOrdID", Type: "STRING"},
			38:  {Number: 38, Name: "OrderQty", Type: "QTY"},
			44:  {Number: 44, Name: "Price", Type: "PRICE"},
			55:  {Number: 55, Name: "Symbol", Type: "STRING"},
			60:  {Number: 60, Name: "TransTime", Type: "UTCTIMESTAMP"},
			100: {Number: 100, Name: "ExDestination", Type: "EXCHANGE"},
			112: {Number: 112, Name: "TestReqID", Type: "STRING"},
			108: {Number: 108, Name: "HeartBtInt", Type: "INT"},
		},
	}

	msg := fixmsg.NewFixMessage()
	// Set mandatory tags
	msg.Set(8, "FIX.4.4")
	msg.Set(9, "10")
	msg.Set(35, "D")
	msg.Set(49, "SENDER")
	msg.Set(56, "TARGET")
	msg.Set(34, "1")
	msg.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))
	msg.Set(10, "123")

	// Set valid dictionary tags
	msg.Set(11, "ORDER1")
	msg.Set(38, "100")
	msg.Set(44, "10.50")
	msg.Set(60, "20260228-12:34:56.789")
	msg.Set(108, "30")

	err := ValidateMessage(msg, s)
	assert.NoError(t, err, "expected message with valid data types to pass")

	// Set invalid INT
	msgInvalidInt := msg
	msgInvalidInt.Set(108, "thirty")
	err = ValidateMessage(msgInvalidInt, s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value for tag 108 (HeartBtInt): expected integer")

	// Reset for next test
	msg.Set(108, "30")

	// Set invalid FLOAT
	msgInvalidFloat := msg
	msgInvalidFloat.Set(44, "abc")
	err = ValidateMessage(msgInvalidFloat, s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value for tag 44 (Price): expected float")

	// Reset for next test
	msg.Set(44, "10.50")

	// Set invalid UTCTIMESTAMP
	msgInvalidTime := msg
	msgInvalidTime.Set(60, "2026-02-28 12:34:56") // Invalid format
	err = ValidateMessage(msgInvalidTime, s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value for tag 60 (TransTime): invalid UTCTimestamp format/length")
}
