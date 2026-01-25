package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmptyProfiler(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name       string
		cpuProfile string
		memProfile string
	}{
		{
			name:       "cpuProfile",
			cpuProfile: filepath.Join(dir, "cpu.prof"),
		},
		{
			name:       "memProfile",
			memProfile: filepath.Join(dir, "mem.prof"),
		},
		{
			name: "empty",
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			p, err := NewProfiler(test.cpuProfile, test.memProfile)
			if err != nil {
				t.Fatalf("newProfiler failed: %v", err)
			}
			if err := p.Stop(); err != nil {
				t.Fatalf("stop failed: %v", err)
			}
		})
	}
}

func TestProfiler(t *testing.T) {
	dir := t.TempDir()
	cpuProfileName := filepath.Join(dir, "cpu.prof")
	memProfileName := filepath.Join(dir, "mem.prof")
	t.Cleanup(func() {
		os.Remove(cpuProfileName)
		os.Remove(memProfileName)
	})

	p, err := NewProfiler(cpuProfileName, memProfileName)
	if err != nil {
		t.Fatalf("newProfiler failed: %v", err)
	}
	if p.CPUProfile == nil {
		t.Fatal("Expected cpuProfile to be non-nil")
	}
	if p.MemProfile != memProfileName {
		t.Fatalf("Expected memProfile to be %s, got %s", memProfileName, p.MemProfile)
	}

	if err := p.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	if _, err := os.Stat(cpuProfileName); os.IsNotExist(err) {
		t.Fatalf("CPU profile file %s was not created", cpuProfileName)
	}

	if _, err := os.Stat(memProfileName); os.IsNotExist(err) {
		t.Fatalf("Memory profile file %s was not created", memProfileName)
	}
}
