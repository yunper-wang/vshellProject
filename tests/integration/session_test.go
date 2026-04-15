package integration

import (
	"testing"
	"time"

	"vshellProject/pkg/session"
)

func TestSessionManagerCreateDestroy(t *testing.T) {
	mgr := session.NewManager(10, 0)

	// Create session
	sess, err := mgr.Create()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if sess.ID == "" {
		t.Error("Session ID should not be empty")
	}

	// Verify session exists
	retrieved, err := mgr.Get(sess.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if retrieved.ID != sess.ID {
		t.Errorf("Session ID mismatch: got %s, want %s", retrieved.ID, sess.ID)
	}

	// Destroy session
	if err := mgr.Destroy(sess.ID); err != nil {
		t.Fatalf("Failed to destroy session: %v", err)
	}

	// Verify session no longer exists
	_, err = mgr.Get(sess.ID)
	if err == nil {
		t.Error("Session should not exist after destroy")
	}
}

func TestSessionManagerMaxSessions(t *testing.T) {
	mgr := session.NewManager(2, 0)

	// Create max sessions
	s1, _ := mgr.Create()
	s2, _ := mgr.Create()

	// Try to create one more
	_, err := mgr.Create()
	if err == nil {
		t.Error("Should fail when max sessions reached")
	}

	// Destroy one and try again
	mgr.Destroy(s1.ID)
	_, err = mgr.Create()
	if err != nil {
		t.Errorf("Should succeed after destroying a session: %v", err)
	}

	// Cleanup
	mgr.Destroy(s2.ID)
}

func TestSessionChannelOperations(t *testing.T) {
	mgr := session.NewManager(10, 0)
	sess, _ := mgr.Create()
	defer mgr.Destroy(sess.ID)

	// Add channel
	ch, err := sess.AddChannel(1, "shell", nil)
	if err != nil {
		t.Fatalf("Failed to add channel: %v", err)
	}

	if ch.ID != 1 {
		t.Errorf("Channel ID mismatch: got %d, want %d", ch.ID, 1)
	}

	// Get channel
	ch2, err := sess.GetChannel(1)
	if err != nil {
		t.Fatalf("Failed to get channel: %v", err)
	}

	if ch2.ID != ch.ID {
		t.Errorf("Channel ID mismatch after get: got %d, want %d", ch2.ID, ch.ID)
	}

	// Remove channel
	if err := sess.RemoveChannel(1); err != nil {
		t.Fatalf("Failed to remove channel: %v", err)
	}

	// Verify channel removed
	_, err = sess.GetChannel(1)
	if err == nil {
		t.Error("Channel should not exist after removal")
	}
}

func TestSessionActivityTracking(t *testing.T) {
	mgr := session.NewManager(10, 0)
	sess, _ := mgr.Create()
	defer mgr.Destroy(sess.ID)

	initialTime := sess.LastActive

	// Wait a small amount
	time.Sleep(10 * time.Millisecond)

	// Update activity
	mgr.UpdateActivity(sess.ID)

	// Get updated session
	updated, _ := mgr.Get(sess.ID)

	if updated.LastActive.Equal(initialTime) {
		t.Error("LastActive should be updated")
	}
}

func TestSessionDataStorage(t *testing.T) {
	mgr := session.NewManager(10, 0)
	sess, _ := mgr.Create()
	defer mgr.Destroy(sess.ID)

	// Store data
	sess.SetData("key1", "value1")
	sess.SetData("key2", 123)

	// Retrieve data
	val1, ok1 := sess.GetData("key1")
	if !ok1 || val1 != "value1" {
		t.Errorf("Data mismatch for key1: got %v, want %v", val1, "value1")
	}

	val2, ok2 := sess.GetData("key2")
	if !ok2 || val2 != 123 {
		t.Errorf("Data mismatch for key2: got %v, want %v", val2, 123)
	}

	// Non-existent key
	_, ok3 := sess.GetData("nonexistent")
	if ok3 {
		t.Error("Should return false for non-existent key")
	}
}

func TestSessionManagerList(t *testing.T) {
	mgr := session.NewManager(10, 0)

	// Create sessions
	s1, _ := mgr.Create()
	s2, _ := mgr.Create()
	s3, _ := mgr.Create()

	// List sessions
	ids := mgr.List()

	if len(ids) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(ids))
	}

	// Verify IDs
	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	if !idMap[s1.ID] || !idMap[s2.ID] || !idMap[s3.ID] {
		t.Error("Expected IDs not found in list")
	}

	// Cleanup
	mgr.Destroy(s1.ID)
	mgr.Destroy(s2.ID)
	mgr.Destroy(s3.ID)
}

func BenchmarkSessionManagerCreateSession(b *testing.B) {
	mgr := session.NewManager(1000, 0)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sess, _ := mgr.Create()
		mgr.Destroy(sess.ID)
	}
}
