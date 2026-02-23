package app

import "testing"

func TestNewBackupOperation(t *testing.T) {
	tests := []struct {
		name       string
		operation  string
		parameters string
	}{
		{
			name:       "with parameters",
			operation:  "BackupAll",
			parameters: "/home/user/docs",
		},
		{
			name:       "empty parameters",
			operation:  "AddDirectory",
			parameters: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := NewBackupOperation(tt.operation, tt.parameters)

			if op.Operation != tt.operation {
				t.Errorf("Operation = %q, want %q", op.Operation, tt.operation)
			}
			if op.Parameters != tt.parameters {
				t.Errorf("Parameters = %q, want %q", op.Parameters, tt.parameters)
			}
			if op.Status != "success" {
				t.Errorf("Status = %q, want %q", op.Status, "success")
			}
			if op.ID != 0 {
				t.Errorf("ID = %d, want 0", op.ID)
			}
		})
	}
}

func TestBackupOperation_Persisted(t *testing.T) {
	tests := []struct {
		name string
		id   int64
		want bool
	}{
		{name: "not persisted when ID is 0", id: 0, want: false},
		{name: "persisted when ID is positive", id: 1, want: true},
		{name: "persisted when ID is large", id: 99999, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := &BackupOperation{ID: tt.id}
			if got := op.Persisted(); got != tt.want {
				t.Errorf("Persisted() = %v, want %v", got, tt.want)
			}
		})
	}
}
