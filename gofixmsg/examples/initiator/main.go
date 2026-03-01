package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wxcuop/gofixmsg/config"
	"github.com/wxcuop/gofixmsg/engine"
	"github.com/wxcuop/gofixmsg/fixmsg"
	"github.com/wxcuop/gofixmsg/network"
	"github.com/wxcuop/gofixmsg/state"
	"github.com/wxcuop/gofixmsg/store"
)

// ExampleApp implements the engine.Application interface for callback hooks.
type ExampleApp struct {
	engine.NoOpApplication
}

func (a *ExampleApp) OnLogon(sessionID string) {
	log.Printf("[ExampleApp] OnLogon: %s", sessionID)
}

func (a *ExampleApp) FromApp(msg *fixmsg.FixMessage, sessionID string) error {
	msgType, _ := msg.Get(35)
	log.Printf("[ExampleApp] FromApp: %s (MsgType=%s)", sessionID, msgType)
	return nil
}

func main() {
	// 1. Load configuration (optional)
	mgr := config.GetManager()
	if err := mgr.Load("config.ini"); err != nil {
		log.Printf("Warning: failed to load config.ini: %v", err)
	}

	// 2. Create the engine (initiator)
	addr := "127.0.0.1:5001"
	if v := mgr.Get("Session", "address"); v != "" {
		addr = v
	}
	
	init := network.NewInitiator(addr)
	fe := engine.NewFixEngine(init)
	fe.SetApplication(&ExampleApp{})
	
	// Enable automatic reconnect
	fe.SetReconnectParams(2*time.Second, 30*time.Second, true)

	// 3. Setup components (State Machine, Store)
	fe.SetupComponents(state.NewStateMachine(), store.NewSQLiteStore())

	// 4. Connect to peer
	log.Printf("Connecting to FIX Acceptor on %s...", addr)
	if err := fe.Connect(); err != nil {
		log.Printf("Initial connection failed: %v. Reconnect loop will handle it.", err)
	}

	// 5. Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	
	log.Printf("Initiator is running. Press Ctrl+C to stop.")
	<-sigCh

	log.Println("Shutting down initiator...")
	fe.Close()
	log.Println("Stopped.")
}
