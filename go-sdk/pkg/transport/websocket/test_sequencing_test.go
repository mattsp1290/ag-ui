package websocket

import (
	"context"
	"testing"
	"time"
)

// Test the new sequencing system to ensure it works correctly
func TestSequencingSystemLight(t *testing.T) {
	RunFastTest(t, func(helper *MinimalTestHelper) {
		server := helper.CreateServer()
		conn := helper.CreateConnection(server.URL())

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := conn.Connect(ctx); err != nil {
			t.Fatal(err)
		}

		message := []byte("sequencing test")
		if err := conn.SendMessage(ctx, message); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSequencingSystemMedium(t *testing.T) {
	RunMediumTest(t, func(helper *MinimalTestHelper) {
		server := helper.CreateServer()

		// Test with 2 connections
		conn1 := helper.CreateConnection(server.URL())
		conn2 := helper.CreateConnection(server.URL())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := conn1.Connect(ctx); err != nil {
			t.Fatal(err)
		}
		if err := conn2.Connect(ctx); err != nil {
			t.Fatal(err)
		}

		// Test concurrent messages
		message1 := []byte("message 1")
		message2 := []byte("message 2")

		if err := conn1.SendMessage(ctx, message1); err != nil {
			t.Fatal(err)
		}
		if err := conn2.SendMessage(ctx, message2); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSequencingSystemHeavy(t *testing.T) {
	RunHeavyTest(t, func(helper *MinimalTestHelper) {
		server := helper.CreateServer()

		// Test with multiple connections (but limited)
		connections := make([]*Connection, 3)
		for i := 0; i < 3; i++ {
			connections[i] = helper.CreateConnection(server.URL())
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Connect all
		for i, conn := range connections {
			if err := conn.Connect(ctx); err != nil {
				t.Fatalf("Connection %d failed: %v", i, err)
			}
		}

		// Send messages from all connections
		for i := 0; i < 5; i++ {
			for j, conn := range connections {
				message := []byte("heavy test message")
				if err := conn.SendMessage(ctx, message); err != nil {
					t.Fatalf("Send failed for connection %d: %v", j, err)
				}
			}
		}
	})
}
