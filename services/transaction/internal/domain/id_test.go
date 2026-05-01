package domain

import "testing"

func TestNewIDLooksLikeUUID(t *testing.T) {
	id := NewID()
	if len(id) != 36 {
		t.Fatalf("NewID() length = %d, want 36", len(id))
	}
	if id[14] != '4' {
		t.Fatalf("NewID() version = %q, want 4", id[14])
	}
}
