package types

import "testing"

func TestPermission_Level(t *testing.T) {
	tests := []struct {
		perm     Permission
		expected int
	}{
		{PermNone, 0},
		{PermView, 1},
		{PermRead, 2},
		{PermWrite, 3},
		{Permission("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.perm), func(t *testing.T) {
			if got := tt.perm.Level(); got != tt.expected {
				t.Errorf("Permission(%q).Level() = %d, want %d", tt.perm, got, tt.expected)
			}
		})
	}
}

func TestPermission_Comparison(t *testing.T) {
	// Test that permission levels are correctly ordered
	if PermNone.Level() >= PermView.Level() {
		t.Error("PermNone should be less than PermView")
	}
	if PermView.Level() >= PermRead.Level() {
		t.Error("PermView should be less than PermRead")
	}
	if PermRead.Level() >= PermWrite.Level() {
		t.Error("PermRead should be less than PermWrite")
	}
}
