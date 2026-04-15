package benchmark

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"vshellProject/pkg/protocol"
	"vshellProject/pkg/session"
)

// BenchmarkProtocolFrameEncoding measures frame encoding performance
func BenchmarkProtocolFrameEncoding(b *testing.B) {
	payload := make([]byte, 1024) // 1KB payload
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame := protocol.NewFrame(protocol.ChannelShell, protocol.TypeShellData, payload)
		_, err := frame.Encode()
		if err != nil {
			b.Fatalf("Encode failed: %v", err)
		}
	}
}

// BenchmarkProtocolFrameDecoding measures frame decoding performance
func BenchmarkProtocolFrameDecoding(b *testing.B) {
	payload := make([]byte, 1024)
	frame := protocol.NewFrame(protocol.ChannelShell, protocol.TypeShellData, payload)
	encoded, _ := frame.Encode()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := protocol.Decode(encoded)
		if err != nil {
			b.Fatalf("Decode failed: %v", err)
		}
	}
}

// BenchmarkSessionManagerCreation measures session manager creation
func BenchmarkSessionManagerCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mgr := session.NewManager(1000, 0)
		_ = mgr.Count()
	}
}

// BenchmarkSessionCreateDestroy measures session lifecycle
func BenchmarkSessionCreateDestroy(b *testing.B) {
	mgr := session.NewManager(10000, 0)
	defer mgr.List()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sess, _ := mgr.Create()
		mgr.Destroy(sess.ID)
	}
}

// BenchmarkConcurrentSessions measures concurrent session operations
func BenchmarkConcurrentSessions(b *testing.B) {
	mgr := session.NewManager(10000, 0)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sess, _ := mgr.Create()
			sess.SetData("key", "value")
			_, _ = sess.GetData("key")
			mgr.Destroy(sess.ID)
		}
	})
}

// StressTestMultipleSessions simulates multiple concurrent sessions
func TestStressMultipleSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	mgr := session.NewManager(1000, 0)
	numSessions := 500
	numOps := 100

	var wg sync.WaitGroup
	wg.Add(numSessions)

	for i := 0; i < numSessions; i++ {
		go func(id int) {
			defer wg.Done()

			sess, err := mgr.Create()
			if err != nil {
				t.Errorf("Failed to create session %d: %v", id, err)
				return
			}

			// Perform operations
			for j := 0; j < numOps; j++ {
				sess.SetData(fmt.Sprintf("key_%d", j), fmt.Sprintf("value_%d", j))
				mgr.UpdateActivity(sess.ID)
			}

			mgr.Destroy(sess.ID)
		}(i)
	}

	done := make(chan bool, 1)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(30 * time.Second):
		t.Fatal("Stress test timed out")
	}
}

// Memory benchmark for protocol operations
func BenchmarkProtocolMemoryUsage(b *testing.B) {
	b.Run("FrameEncoding", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			payload := make([]byte, 4096)
			frame := protocol.NewFrame(protocol.ChannelFile, protocol.TypeFileData, payload)
			_, _ = frame.Encode()
		}
	})

	b.Run("SessionData", func(b *testing.B) {
		mgr := session.NewManager(1000, 0)
		for i := 0; i < b.N; i++ {
			sess, _ := mgr.Create()
			for j := 0; j < 100; j++ {
				sess.SetData(fmt.Sprintf("key_%d", j), make([]byte, 1024))
			}
			mgr.Destroy(sess.ID)
		}
	})
}

// Throughput test for shell data simulation
func BenchmarkShellDataThroughput(b *testing.B) {
	// Simulate terminal output
	terminalOutput := make([]byte, 4096)
	for i := range terminalOutput {
		terminalOutput[i] = 'a' + byte(i%26)
	}

	mgr := session.NewManager(100, 0)
	sess, _ := mgr.Create()
	_ = sess.AddChannel(protocol.ChannelShell, "shell", nil)

	b.ResetTimer()
	b.SetBytes(int64(len(terminalOutput)))

	for i := 0; i < b.N; i++ {
		frame := protocol.NewFrame(protocol.ChannelShell, protocol.TypeShellData, terminalOutput)
		_, _ = frame.Encode()
	}
}
