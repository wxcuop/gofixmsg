package handler_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/gofixmsg/handler"
)

type mockHandler struct{ called bool }

func (m *mockHandler) Handle(msgType string, body []byte) error { m.called = true; return nil }

func TestProcessor(t *testing.T) {
	p := handler.NewProcessor()
	m := &mockHandler{}
	p.Register("D", m)
	require.NoError(t, p.Process("D", []byte("body")))
	require.True(t, m.called)
	// unknown type
	reqErr := p.Process("Z", []byte("x"))
	require.Error(t, reqErr)
}
