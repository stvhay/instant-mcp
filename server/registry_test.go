package server

import (
	"fmt"
	"sync"
	"testing"

	"github.com/hays/instant-mcp/models"
)

func testCommand(name string) models.Command {
	return models.Command{
		Name:        name,
		Exec:        "/usr/bin/echo",
		Description: "Test command",
		Args: map[string]models.Arg{
			"msg": {Type: "string", Description: "Message", Required: true},
		},
		Timeout: "30s",
	}
}

func TestRegistryAdd(t *testing.T) {
	r := NewRegistry()

	cmd := testCommand("hello")
	if err := r.Add(cmd); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if r.Len() != 1 {
		t.Fatalf("expected 1 command, got %d", r.Len())
	}
}

func TestRegistryAddDuplicate(t *testing.T) {
	r := NewRegistry()

	cmd := testCommand("hello")
	r.Add(cmd)

	err := r.Add(cmd)
	if err == nil {
		t.Fatal("expected error on duplicate add")
	}
}

func TestRegistryAddInvalidName(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		name    string
		wantErr bool
	}{
		{"", true},
		{"123start", true},
		{"has-dash", true},
		{"has space", true},
		{"valid_name", false},
		{"CamelCase", false},
		{"a1", false},
	}

	for _, tt := range tests {
		cmd := testCommand("placeholder")
		cmd.Name = tt.name
		err := r.Add(cmd)
		if (err != nil) != tt.wantErr {
			t.Errorf("Add(%q): err=%v, wantErr=%v", tt.name, err, tt.wantErr)
		}
		// Clean up successful adds
		if err == nil {
			r.Remove(tt.name)
		}
	}
}

func TestRegistryAddMissingExec(t *testing.T) {
	r := NewRegistry()

	cmd := models.Command{Name: "noexec"}
	err := r.Add(cmd)
	if err == nil {
		t.Fatal("expected error when exec is missing")
	}
}

func TestRegistryAddInvalidArgType(t *testing.T) {
	r := NewRegistry()

	cmd := testCommand("badarg")
	cmd.Args = map[string]models.Arg{
		"x": {Type: "invalid"},
	}
	err := r.Add(cmd)
	if err == nil {
		t.Fatal("expected error on invalid arg type")
	}
}

func TestRegistryAddInvalidTimeout(t *testing.T) {
	r := NewRegistry()

	cmd := testCommand("badtimeout")
	cmd.Timeout = "forever"
	err := r.Add(cmd)
	if err == nil {
		t.Fatal("expected error on invalid timeout")
	}
}

func TestRegistryRemove(t *testing.T) {
	r := NewRegistry()

	r.Add(testCommand("hello"))
	if err := r.Remove("hello"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if r.Len() != 0 {
		t.Fatal("expected 0 commands after remove")
	}
}

func TestRegistryRemoveNotFound(t *testing.T) {
	r := NewRegistry()

	err := r.Remove("nonexistent")
	if err == nil {
		t.Fatal("expected error removing nonexistent command")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()

	orig := testCommand("hello")
	r.Add(orig)

	cmd, err := r.Get("hello")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if cmd.Name != orig.Name || cmd.Exec != orig.Exec {
		t.Fatalf("Get returned wrong command: %+v", cmd)
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error getting nonexistent command")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()

	r.Add(testCommand("alpha"))
	r.Add(testCommand("beta"))
	r.Add(testCommand("gamma"))

	cmds := r.List()
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
}

func TestRegistryListEmpty(t *testing.T) {
	r := NewRegistry()

	cmds := r.List()
	if len(cmds) != 0 {
		t.Fatalf("expected 0 commands, got %d", len(cmds))
	}
}

func TestRegistryUpdate(t *testing.T) {
	r := NewRegistry()

	r.Add(testCommand("hello"))

	updated := testCommand("hello")
	updated.Description = "Updated description"
	if err := r.Update("hello", updated); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	cmd, _ := r.Get("hello")
	if cmd.Description != "Updated description" {
		t.Fatalf("Update didn't apply: %s", cmd.Description)
	}
}

func TestRegistryUpdateNotFound(t *testing.T) {
	r := NewRegistry()

	err := r.Update("nonexistent", testCommand("nonexistent"))
	if err == nil {
		t.Fatal("expected error updating nonexistent command")
	}
}

func TestRegistryUpdateRename(t *testing.T) {
	r := NewRegistry()

	r.Add(testCommand("old_name"))

	renamed := testCommand("new_name")
	if err := r.Update("old_name", renamed); err != nil {
		t.Fatalf("Update (rename) failed: %v", err)
	}

	if _, err := r.Get("old_name"); err == nil {
		t.Fatal("old name should not exist")
	}
	if _, err := r.Get("new_name"); err != nil {
		t.Fatal("new name should exist")
	}
}

func TestRegistrySnapshotAndLoad(t *testing.T) {
	r := NewRegistry()

	r.Add(testCommand("alpha"))
	r.Add(testCommand("beta"))

	snap := r.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("snapshot has %d commands, want 2", len(snap))
	}

	// Load into new registry
	r2 := NewRegistry()
	r2.Load(snap)

	if r2.Len() != 2 {
		t.Fatalf("loaded registry has %d commands, want 2", r2.Len())
	}

	cmd, err := r2.Get("alpha")
	if err != nil || cmd.Exec != "/usr/bin/echo" {
		t.Fatalf("loaded command doesn't match: %+v %v", cmd, err)
	}
}

func TestRegistryConcurrency(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	// Concurrent adds
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cmd := testCommand("placeholder")
			cmd.Name = fmt.Sprintf("cmd_%d", n)
			r.Add(cmd)
		}(i)
	}
	wg.Wait()

	if r.Len() != 100 {
		t.Fatalf("expected 100 commands, got %d", r.Len())
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Get(fmt.Sprintf("cmd_%d", n))
			r.List()
		}(i)
	}
	wg.Wait()
}

