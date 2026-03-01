package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wxcuop/gofixmsg/config"
	"github.com/wxcuop/gofixmsg/engine"
	"github.com/wxcuop/gofixmsg/fixmsg"
)

// ExampleApp implements the engine.Application interface for callback hooks.
type ExampleApp struct {
	engine.NoOpApplication
}

func (a *ExampleApp) OnCreate(sessionID string) {
	log.Printf("[ExampleApp] OnCreate: %s", sessionID)
}

func (a *ExampleApp) OnLogon(sessionID string) {
	log.Printf("[ExampleApp] OnLogon: %s", sessionID)
}

func (a *ExampleApp) OnLogout(sessionID string) {
	log.Printf("[ExampleApp] OnLogout: %s", sessionID)
}

func (a *ExampleApp) FromApp(msg *fixmsg.FixMessage, sessionID string) error {
	msgType, _ := msg.Get(35)
	log.Printf("[ExampleApp] FromApp: %s (MsgType=%s)", sessionID, msgType)
	return nil
}

func (a *ExampleApp) OnMessage(msg *fixmsg.FixMessage, sessionID string) {
	msgType, _ := msg.Get(35)
	log.Printf("[ExampleApp] OnMessage: %s (MsgType=%s)", sessionID, msgType)
}

func main() {
	// 1. Load configuration (optional)
	mgr := config.GetManager()
	if err := mgr.Load("config.ini"); err != nil {
		log.Printf("Warning: failed to load config.ini: %v", err)
	}

	// 2. Create the multi-session engine (acceptor)
	addr := "0.0.0.0:5001"
	if v := mgr.Get("Session", "bind_address"); v != "" {
		addr = v
	}
	
	m := engine.NewMultiSessionEngine(addr)
	m.SetApplication(&ExampleApp{})

	// 3. Start the engine
	log.Printf("Starting FIX Acceptor on %s...", addr)
	if err := m.Start(); err != nil {
		log.Fatalf("Failed to start acceptor: %v", err)
	}

	// 4. Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	
	log.Printf("Acceptor is running. Press Ctrl+C to stop.")
	<-sigCh

	log.Println("Shutting down acceptor...")
	m.Stop()
	log.Println("Stopped.")
}
