package handler

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/state"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

// MockStore for testing handlers
type MockStore struct {
	messages map[string]*store.Message
	outSeqs  map[string]int
	inSeqs   map[string]int
}

func NewMockStore() *MockStore {
	return &MockStore{
		messages: make(map[string]*store.Message),
		outSeqs:  make(map[string]int),
		inSeqs:   make(map[string]int),
	}
}
func (m *MockStore) Init(path string) error { return nil }
func (m *MockStore) SaveMessage(msg *store.Message) error {
	key := msg.BeginString + ":" + msg.SenderCompID + ":" + msg.TargetCompID + ":" + strconv.Itoa(msg.MsgSeqNum)
	m.messages[key] = msg
	return nil
}
func (m *MockStore) GetMessage(begin, sender, target string, seq int) (*store.Message, error) {
	key := begin + ":" + sender + ":" + target + ":" + strconv.Itoa(seq)
	if msg, ok := m.messages[key]; ok {
		return msg, nil
	}
	return nil, nil // not found
}
func (m *MockStore) SaveSessionSeq(sessionID string, outSeq int, inSeq int) error {
	m.outSeqs[sessionID] = outSeq
	m.inSeqs[sessionID] = inSeq
	return nil
}
func (m *MockStore) GetSessionSeq(sessionID string) (int, int, error) {
	return m.outSeqs[sessionID], m.inSeqs[sessionID], nil
}

// DummyConn for capturing session writes
type DummyConn struct {
	net.Conn
	written [][]byte
}
func (d *DummyConn) Write(b []byte) (n int, err error) {
	cpy := make([]byte, len(b))
	copy(cpy, b)
	d.written = append(d.written, cpy)
	return len(b), nil
}
func (d *DummyConn) Close() error { return nil }
func (d *DummyConn) Read(b []byte) (n int, err error) {
	// block slightly to simulate idle read, but return EOF eventually to allow clean shutdown
	time.Sleep(10 * time.Millisecond)
	return 0, net.ErrClosed
}
func (d *DummyConn) SetReadDeadline(t time.Time) error { return nil }
func (d *DummyConn) SetWriteDeadline(t time.Time) error { return nil }

// testSeqMgr implements SeqMgrI for testing
type testSeqMgr struct {
	incoming int
	outgoing int
}

func (s *testSeqMgr) Incoming() int { return s.incoming }
func (s *testSeqMgr) Outgoing() int { return s.outgoing }
func (s *testSeqMgr) SetIncoming(i int) { s.incoming = i }
func (s *testSeqMgr) SetOutgoing(o int) error { s.outgoing = o; return nil }
func (s *testSeqMgr) IncrementIncoming() int { s.incoming++; return s.incoming }
func (s *testSeqMgr) IncrementOutgoing() (int, error) { s.outgoing++; return s.outgoing, nil }

// testEngine implements EngineI for testing
type testEngine struct {
	seqMgr        SeqMgrI
	app           Application
	sid           string
	capturedWrites [][]byte
}

func (e *testEngine) SendMessage(msg *fixmsg.FixMessage) error {
	wire, err := msg.ToWire()
	if err != nil {
		return err
	}
	e.capturedWrites = append(e.capturedWrites, wire)
	return nil
}
func (e *testEngine) SessionSend(b []byte) error {
	cpy := make([]byte, len(b))
	copy(cpy, b)
	e.capturedWrites = append(e.capturedWrites, cpy)
	return nil
}
func (e *testEngine) GetApp() Application { return e.app }
func (e *testEngine) GetSessionID() string { return e.sid }
func (e *testEngine) GetSeqMgr() SeqMgrI { return e.seqMgr }

func TestResendRequestHandler_Hardening(t *testing.T) {
	mockStore := NewMockStore()
	sm := state.NewStateMachine()
	
	fe := &testEngine{
		seqMgr:         &testSeqMgr{},
		sid:            "session1",
		capturedWrites: [][]byte{},
	}

	ctx := &HandlerContext{
		SM:     sm,
		Store:  mockStore,
		Engine: fe,
	}

	proc := NewProcessor()
	RegisterDefaultHandlers(proc, ctx)

	// Populate the mock store with some messages
	// 1. Admin message (Heartbeat) - seq 1
	// 2. App message (NewOrderSingle) - seq 2
	// 3. Missing message - seq 3
	// 4. App message (ExecutionReport) - seq 4

	origTime := "20260228-10:00:00.000"

	hbMsg := fixmsg.NewFixMessageFromMap(map[int]string{8: "FIX.4.4", 35: "0", 49: "TARGET", 56: "SENDER", 34: "1", 52: origTime})
	hbBytes, _ := hbMsg.ToWire()
	_ = mockStore.SaveMessage(&store.Message{BeginString: "FIX.4.4", SenderCompID: "TARGET", TargetCompID: "SENDER", MsgSeqNum: 1, MsgType: "0", Body: hbBytes})

	appMsg1 := fixmsg.NewFixMessageFromMap(map[int]string{8: "FIX.4.4", 35: "D", 49: "TARGET", 56: "SENDER", 34: "2", 52: origTime, 11: "ORD1"})
	appBytes1, _ := appMsg1.ToWire()
	_ = mockStore.SaveMessage(&store.Message{BeginString: "FIX.4.4", SenderCompID: "TARGET", TargetCompID: "SENDER", MsgSeqNum: 2, MsgType: "D", Body: appBytes1})

	appMsg2 := fixmsg.NewFixMessageFromMap(map[int]string{8: "FIX.4.4", 35: "8", 49: "TARGET", 56: "SENDER", 34: "4", 52: origTime, 37: "EXEC1"})
	appBytes2, _ := appMsg2.ToWire()
	_ = mockStore.SaveMessage(&store.Message{BeginString: "FIX.4.4", SenderCompID: "TARGET", TargetCompID: "SENDER", MsgSeqNum: 4, MsgType: "8", Body: appBytes2})

	// Construct incoming ResendRequest (seqs 1 to 4)
	rrMsg := fixmsg.NewFixMessageFromMap(map[int]string{
		8: "FIX.4.4",
		35: "2",
		49: "SENDER", // the peer asking us
		56: "TARGET", // us
		34: "100",
		52: origTime,
		7: "1", // begin seq no
		16: "4", // end seq no
	})
	rrMsg.SetLenAndChecksum()

	err := proc.Process(rrMsg)
	assert.NoError(t, err)

	// Wait briefly for async processing
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 4, len(fe.capturedWrites), "Should have sent 4 messages in response")


	// Verify msg 1: Should be SequenceReset (GapFill) for seq 1 because it's an admin message
	msg1 := fixmsg.NewFixMessage()
	_ = msg1.LoadFix(fe.capturedWrites[0])
	mType1, _ := msg1.Get(35)
	assert.Equal(t, "4", mType1, "Seq 1 should be SequenceReset")
	gf1, _ := msg1.Get(123)
	assert.Equal(t, "Y", gf1)
	newSeq1, _ := msg1.Get(36)
	assert.Equal(t, "2", newSeq1) // next seq
	seqnum1, _ := msg1.Get(34)
	assert.Equal(t, "1", seqnum1, "Should retain original seqnum 1")
	pd1, _ := msg1.Get(43)
	assert.Equal(t, "Y", pd1, "Should have PossDupFlag=Y")

	// Verify msg 2: Should be replayed NewOrderSingle (D)
	msg2 := fixmsg.NewFixMessage()
	_ = msg2.LoadFix(fe.capturedWrites[1])
	mType2, _ := msg2.Get(35)
	assert.Equal(t, "D", mType2, "Seq 2 should be replayed app message")
	seqnum2, _ := msg2.Get(34)
	assert.Equal(t, "2", seqnum2)
	pd2, _ := msg2.Get(43)
	assert.Equal(t, "Y", pd2, "Replayed message should have PossDupFlag=Y")
	origST2, _ := msg2.Get(122)
	assert.Equal(t, origTime, origST2, "OrigSendingTime should match original SendingTime")
	st2, _ := msg2.Get(52)
	assert.NotEqual(t, origTime, st2, "SendingTime should be updated")

	// Verify msg 3: Should be GapFill for missing seq 3
	msg3 := fixmsg.NewFixMessage()
	_ = msg3.LoadFix(fe.capturedWrites[2])
	mType3, _ := msg3.Get(35)
	assert.Equal(t, "4", mType3, "Seq 3 should be GapFill because missing")
	seqnum3, _ := msg3.Get(34)
	assert.Equal(t, "3", seqnum3)
	newSeq3, _ := msg3.Get(36)
	assert.Equal(t, "4", newSeq3)

	// Verify msg 4: Should be replayed ExecutionReport (8)
	msg4 := fixmsg.NewFixMessage()
	_ = msg4.LoadFix(fe.capturedWrites[3])
	mType4, _ := msg4.Get(35)
	assert.Equal(t, "8", mType4, "Seq 4 should be replayed app message")
	seqnum4, _ := msg4.Get(34)
	assert.Equal(t, "4", seqnum4)
	pd4, _ := msg4.Get(43)
	assert.Equal(t, "Y", pd4)
}

func TestLogonHandler_ResetSeqNumFlag(t *testing.T) {
	mockStore := NewMockStore()
	sm := state.NewStateMachine()
	
	fe := &testEngine{
		seqMgr:         &testSeqMgr{incoming: 100, outgoing: 100},
		sid:            "session1",
		capturedWrites: [][]byte{},
	}

	ctx := &HandlerContext{
		SM:     sm,
		Store:  mockStore,
		Engine: fe,
	}

	proc := NewProcessor()
	RegisterDefaultHandlers(proc, ctx)

	// Send Logon with ResetSeqNumFlag=Y
	logonMsg := fixmsg.NewFixMessageFromMap(map[int]string{
		8: "FIX.4.4",
		35: "A",
		49: "SENDER",
		56: "TARGET",
		34: "1",
		52: time.Now().UTC().Format("20060102-15:04:05.000"),
		141: "Y",
	})
	logonMsg.SetLenAndChecksum()

	err := proc.Process(logonMsg)
	assert.NoError(t, err)

	outSeq := fe.seqMgr.Outgoing()
	assert.Equal(t, 1, outSeq, "Outgoing sequence should be reset to 1")
	inSeq := fe.seqMgr.Incoming()
	assert.Equal(t, 1, inSeq, "Incoming sequence should be reset to 1")
}

func TestResendRequestHandler_EndSeqNoZero(t *testing.T) {
	mockStore := NewMockStore()
	sm := state.NewStateMachine()
	
	fe := &testEngine{
		seqMgr:         &testSeqMgr{outgoing: 10},
		sid:            "session1",
		capturedWrites: [][]byte{},
	}

	ctx := &HandlerContext{SM: sm, Store: mockStore, Engine: fe}
	proc := NewProcessor()
	RegisterDefaultHandlers(proc, ctx)

	// Peer requests 5 to 0 (all messages after 5)
	rrMsg := fixmsg.NewFixMessageFromMap(map[int]string{
		8: "FIX.4.4", 35: "2", 49: "SENDER", 56: "TARGET", 34: "100", 52: "20260228-10:00:00.000",
		7: "5", 16: "0",
	})
	rrMsg.SetLenAndChecksum()

	err := proc.Process(rrMsg)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	// Outgoing was 10, so messages 5,6,7,8,9,10 should be replayed (or gap-filled)
	// In our mock store they are missing, so 6 messages should be sent.
	assert.Equal(t, 6, len(fe.capturedWrites), "Should have replayed 6 messages (5 to 10)")
}

func TestLogonHandler_ResetSeqNumFlagNo(t *testing.T) {
	mockStore := NewMockStore()
	sm := state.NewStateMachine()
	
	fe := &testEngine{
		seqMgr:         &testSeqMgr{incoming: 100, outgoing: 100},
		sid:            "session1",
		capturedWrites: [][]byte{},
	}

	ctx := &HandlerContext{SM: sm, Store: mockStore, Engine: fe}
	proc := NewProcessor()
	RegisterDefaultHandlers(proc, ctx)

	// Logon with ResetSeqNumFlag=N
	logonMsg := fixmsg.NewFixMessageFromMap(map[int]string{
		8: "FIX.4.4", 35: "A", 49: "SENDER", 56: "TARGET", 34: "1", 52: "20260228-10:00:00.000",
		141: "N",
	})
	logonMsg.SetLenAndChecksum()

	err := proc.Process(logonMsg)
	assert.NoError(t, err)

	assert.Equal(t, 100, fe.seqMgr.Outgoing(), "Outgoing sequence should NOT be reset")
	assert.Equal(t, 100, fe.seqMgr.Incoming(), "Incoming sequence should NOT be reset")
}
