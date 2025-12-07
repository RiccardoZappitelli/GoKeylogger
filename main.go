package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RiccardoZappitelli/GoKeylogger/keylogger"
)

func main() {
	kl := keylogger.NewKeyLogger(func(event keylogger.KeyEvent) {
		fmt.Printf("Key: %s | Shift: %v | Ctrl: %v | Alt: %v\n",
			event.Key, event.IsShift, event.IsCtrl, event.IsAlt)
	})

	if err := kl.Start(); err != nil {
		fmt.Printf("Failed to start keylogger: %v\n", err)
		return
	}
	defer kl.Stop()

	fmt.Println("Keylogger started. Press Ctrl+C to stop.")

	// Run as goroutine
	go func() {
		time.Sleep(30 * time.Second)
		fmt.Println("Auto-stopping after 30 seconds...")
		kl.Stop()
	}()

	// Alternative: Process keys from channel
	go func() {
		for event := range kl.GetKeyChannel() {
			if !event.IsSpecial {
				fmt.Printf("Normal key: %s\n", event.Key)
			}
		}
	}()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nStopping keylogger...")
}
