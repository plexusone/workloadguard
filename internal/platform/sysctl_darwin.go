//go:build darwin

package platform

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

// LoadAverage contains the system load averages.
type LoadAverage struct {
	Load1  float64
	Load5  float64
	Load15 float64
}

// GetLoadAverage returns the system load averages.
func GetLoadAverage() (*LoadAverage, error) {
	// Use SysctlRaw to get the binary struct.
	data, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		return nil, fmt.Errorf("sysctl vm.loadavg: %w", err)
	}

	// The struct is:
	// - 3 x uint32 (12 bytes) for ldavg
	// - 1 x int64 (8 bytes) for fscale on 64-bit systems
	// Total: 20 bytes minimum (may have padding)
	if len(data) < 20 {
		return nil, fmt.Errorf("unexpected vm.loadavg size: %d", len(data))
	}

	// Parse the load averages (little-endian).
	ldavg1 := binary.LittleEndian.Uint32(data[0:4])
	ldavg5 := binary.LittleEndian.Uint32(data[4:8])
	ldavg15 := binary.LittleEndian.Uint32(data[8:12])

	// Parse fscale. On ARM64 macOS, this is at offset 16 (with 4 bytes padding).
	// On x86_64, it might be at offset 12.
	var fscale int64

	// Try to read fscale from the right location.
	// ARM64 has 4 bytes padding after the 3 uint32s.
	if len(data) >= 24 {
		fscale = int64(binary.LittleEndian.Uint64(data[16:24])) //nolint:gosec // safe: fscale is never negative in kernel data
	} else if len(data) >= 20 {
		fscale = int64(binary.LittleEndian.Uint64(data[12:20])) //nolint:gosec // safe: fscale is never negative in kernel data
	}

	if fscale == 0 {
		// Default scale factor if we couldn't read it.
		fscale = 2048
	}

	return &LoadAverage{
		Load1:  float64(ldavg1) / float64(fscale),
		Load5:  float64(ldavg5) / float64(fscale),
		Load15: float64(ldavg15) / float64(fscale),
	}, nil
}

// GetCPUCount returns the number of logical CPUs.
func GetCPUCount() (int, error) {
	count, err := unix.SysctlUint32("hw.ncpu")
	if err != nil {
		return 0, fmt.Errorf("sysctl hw.ncpu: %w", err)
	}
	return int(count), nil
}

// GetPhysicalMemory returns the total physical memory in bytes.
func GetPhysicalMemory() (uint64, error) {
	mem, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0, fmt.Errorf("sysctl hw.memsize: %w", err)
	}
	return mem, nil
}

// MemoryStats contains system memory statistics.
type MemoryStats struct {
	Total     uint64
	PageSize  uint64
	FreePages uint64
	FreeBytes uint64
}

// GetMemoryStats returns system memory statistics.
func GetMemoryStats() (*MemoryStats, error) {
	total, err := GetPhysicalMemory()
	if err != nil {
		return nil, err
	}

	pageSize, err := unix.SysctlUint32("vm.pagesize")
	if err != nil {
		return nil, fmt.Errorf("sysctl vm.pagesize: %w", err)
	}

	// vm.page_free_count requires raw sysctl call.
	freePages, err := getVMStat("vm.page_free_count")
	if err != nil {
		// Non-fatal; some macOS versions may not expose this.
		freePages = 0
	}

	return &MemoryStats{
		Total:     total,
		PageSize:  uint64(pageSize),
		FreePages: freePages,
		FreeBytes: freePages * uint64(pageSize),
	}, nil
}

func getVMStat(name string) (uint64, error) {
	// SysctlRaw returns the raw bytes.
	data, err := unix.SysctlRaw(name)
	if err != nil {
		return 0, err
	}

	if len(data) < 4 {
		return 0, fmt.Errorf("unexpected sysctl result length: %d", len(data))
	}

	// Interpret as uint32 (little-endian on macOS ARM64/x86_64).
	val := binary.LittleEndian.Uint32(data)
	return uint64(val), nil
}
