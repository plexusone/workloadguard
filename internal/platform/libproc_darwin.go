//go:build darwin

// Package platform provides macOS-specific system interfaces.
package platform

/*
#include <libproc.h>
#include <sys/proc_info.h>
#include <stdlib.h>

// proc_pidpath wrapper that handles the buffer.
int get_proc_pidpath(int pid, char *buffer, int buffersize) {
    return proc_pidpath(pid, buffer, buffersize);
}
*/
import "C"

import (
	"fmt"
	"path/filepath"
	"unsafe"
)

// MaxPIDs is the maximum number of PIDs to retrieve.
const MaxPIDs = 32768

// Process represents a process from libproc.
type Process struct {
	PID  int
	PPID int
	Name string
	Path string
}

// ListPIDs returns all process IDs on the system.
func ListPIDs() ([]int, error) {
	// First call to get the buffer size needed.
	bufferSize := C.proc_listpids(C.PROC_ALL_PIDS, 0, nil, 0)
	if bufferSize <= 0 {
		return nil, fmt.Errorf("proc_listpids failed to get size")
	}

	// Allocate buffer for PIDs.
	pidCount := int(bufferSize) / C.sizeof_int
	if pidCount > MaxPIDs {
		pidCount = MaxPIDs
	}

	buffer := make([]C.int, pidCount)

	// Second call to get actual PIDs.
	bufferSize = C.proc_listpids(
		C.PROC_ALL_PIDS,
		0,
		unsafe.Pointer(&buffer[0]),
		C.int(len(buffer))*C.sizeof_int,
	)
	if bufferSize <= 0 {
		return nil, fmt.Errorf("proc_listpids failed")
	}

	count := int(bufferSize) / C.sizeof_int
	pids := make([]int, 0, count)

	for i := 0; i < count; i++ {
		if buffer[i] > 0 {
			pids = append(pids, int(buffer[i]))
		}
	}

	return pids, nil
}

// GetProcessInfo retrieves information about a process.
func GetProcessInfo(pid int) (*Process, error) {
	var taskInfo C.struct_proc_taskallinfo

	size := C.proc_pidinfo(
		C.int(pid),
		C.PROC_PIDTASKALLINFO,
		0,
		unsafe.Pointer(&taskInfo),
		C.int(unsafe.Sizeof(taskInfo)),
	)

	if size <= 0 {
		return nil, fmt.Errorf("proc_pidinfo failed for pid %d", pid)
	}

	// Get process path.
	pathBuffer := make([]C.char, C.PROC_PIDPATHINFO_MAXSIZE)
	pathLen := C.get_proc_pidpath(
		C.int(pid),
		&pathBuffer[0],
		C.PROC_PIDPATHINFO_MAXSIZE,
	)

	var path string
	if pathLen > 0 {
		path = C.GoString(&pathBuffer[0])
	}

	// Extract process name from comm field.
	name := C.GoString(&taskInfo.pbsd.pbi_comm[0])

	return &Process{
		PID:  pid,
		PPID: int(taskInfo.pbsd.pbi_ppid),
		Name: name,
		Path: path,
	}, nil
}

// GetProcessName returns just the process name (basename of executable).
func GetProcessName(pid int) (string, error) {
	pathBuffer := make([]C.char, C.PROC_PIDPATHINFO_MAXSIZE)
	pathLen := C.get_proc_pidpath(
		C.int(pid),
		&pathBuffer[0],
		C.PROC_PIDPATHINFO_MAXSIZE,
	)

	if pathLen <= 0 {
		// Fallback to proc_name.
		nameBuffer := make([]C.char, C.PROC_PIDPATHINFO_MAXSIZE)
		nameLen := C.proc_name(C.int(pid), unsafe.Pointer(&nameBuffer[0]), C.uint32_t(len(nameBuffer)))
		if nameLen <= 0 {
			return "", fmt.Errorf("failed to get name for pid %d", pid)
		}
		return C.GoString(&nameBuffer[0]), nil
	}

	path := C.GoString(&pathBuffer[0])
	return filepath.Base(path), nil
}

// ProcessResourceUsage contains CPU and memory usage for a process.
type ProcessResourceUsage struct {
	PID            int
	PPID           int
	Name           string
	CPUUsage       float64 // percentage
	ResidentMemory uint64  // bytes
	VirtualMemory  uint64  // bytes
}

// GetProcessResourceUsage retrieves resource usage for a process.
func GetProcessResourceUsage(pid int) (*ProcessResourceUsage, error) {
	var taskInfo C.struct_proc_taskallinfo

	size := C.proc_pidinfo(
		C.int(pid),
		C.PROC_PIDTASKALLINFO,
		0,
		unsafe.Pointer(&taskInfo),
		C.int(unsafe.Sizeof(taskInfo)),
	)

	if size <= 0 {
		return nil, fmt.Errorf("proc_pidinfo failed for pid %d", pid)
	}

	name := C.GoString(&taskInfo.pbsd.pbi_comm[0])

	// CPU time is in nanoseconds; we return total time here.
	// For percentage, the daemon tracks delta over time.
	totalTime := uint64(taskInfo.ptinfo.pti_total_user) +
		uint64(taskInfo.ptinfo.pti_total_system)

	return &ProcessResourceUsage{
		PID:            pid,
		PPID:           int(taskInfo.pbsd.pbi_ppid),
		Name:           name,
		CPUUsage:       float64(totalTime) / 1e9, // seconds of CPU time
		ResidentMemory: uint64(taskInfo.ptinfo.pti_resident_size),
		VirtualMemory:  uint64(taskInfo.ptinfo.pti_virtual_size),
	}, nil
}
