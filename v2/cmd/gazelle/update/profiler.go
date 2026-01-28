package update

import (
	"os"
	"runtime"
	"runtime/pprof"
)

type Profiler struct {
	CPUProfile *os.File
	MemProfile string
}

// NewProfiler creates a profiler that writes to the given files.
// it returns an empty profiler if both files are empty.
// so that stop() will never fail.
func NewProfiler(cpuProfile, memProfile string) (Profiler, error) {
	if cpuProfile == "" {
		return Profiler{
			MemProfile: memProfile,
		}, nil
	}

	f, err := os.Create(cpuProfile)
	if err != nil {
		return Profiler{}, err
	}
	pprof.StartCPUProfile(f)

	return Profiler{
		CPUProfile: f,
		MemProfile: memProfile,
	}, nil
}

func (p *Profiler) Stop() error {
	if p.CPUProfile != nil {
		pprof.StopCPUProfile()
		p.CPUProfile.Close()
	}

	if p.MemProfile == "" {
		return nil
	}

	f, err := os.Create(p.MemProfile)
	if err != nil {
		return err
	}
	defer f.Close()
	runtime.GC()
	return pprof.WriteHeapProfile(f)
}
