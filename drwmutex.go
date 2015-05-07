// package drwmutex provides a DRWMutex, a distributed RWMutex for use when
// there are many readers spread across many cores, and relatively few cores.
// DRWMutex is meant as an almost drop-in replacement for sync.RWMutex.
package drwmutex

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// cpu returns a unique identifier for the core the current goroutine is
// executing on. This function is platform dependent, and is implemented in
// cpu_*.s.
func cpu() uint64

// cpus maps (non-consecutive) CPUID values to integer indices.
var cpus map[uint64]int

// init will construct the cpus map so that CPUIDs can be looked up to
// determine a particular core's lock index.
func init() {
	cpus = make(map[uint64]int)

	var aff uint64
	syscall.Syscall(syscall.SYS_SCHED_GETAFFINITY, uintptr(0), unsafe.Sizeof(aff), uintptr(unsafe.Pointer(&aff)))

	n := 0
	start := time.Now()

	var mask uint64 = 1
Outer:
	// move the process to one core at the time, each time determining that
	// core's CPU ID
	for {
		// find the next 1 in the affinity bitmap
		for (aff & mask) == 0 {
			mask <<= 1
			// end if the mask wraps around, or if we've moved past
			// the highest bit in the affinity bitmap
			if mask == 0 || mask > aff {
				break Outer
			}
		}

		// lock the process to this core
		ret, _, err := syscall.Syscall(syscall.SYS_SCHED_SETAFFINITY, uintptr(0), unsafe.Sizeof(mask), uintptr(unsafe.Pointer(&mask)))
		if ret != 0 {
			panic(err.Error())
		}

		// what CPU do we have?
		c := cpu()

		if oldn, ok := cpus[c]; ok {
			fmt.Fprintln(os.Stderr, "cpu", n, "==", oldn, "-- both have CPUID", c)
		}

		cpus[c] = n
		mask <<= 1
		n++
	}

	fmt.Fprintf(os.Stderr, "%d/%d cpus found in %v: %v\n", len(cpus), runtime.NumCPU(), time.Now().Sub(start), cpus)

	// give process access to all the cores it had access to initially
	ret, _, err := syscall.Syscall(syscall.SYS_SCHED_SETAFFINITY, uintptr(0), unsafe.Sizeof(aff), uintptr(unsafe.Pointer(&aff)))
	if ret != 0 {
		panic(err.Error())
	}
}

type DRWMutex []sync.RWMutex

// New returns a new, unlocked, distributed RWMutex.
func New() DRWMutex {
	return make(DRWMutex, runtime.GOMAXPROCS(0))
}

// Lock takes out an exclusive writer lock similar to sync.Mutex.Lock.
// A writer lock also excludes all readers.
func (mx DRWMutex) Lock() {
	for core := range mx {
		mx[core].Lock()
	}
}

// Unlock releases an exclusive writer lock similar to sync.Mutex.Unlock.
func (mx DRWMutex) Unlock() {
	for core := range mx {
		mx[core].Unlock()
	}
}

// RLocker returns a sync.Locker presenting Lock() and Unlock() methods that
// take and release a non-exclusive *reader* lock. Note that this call may be
// relatively slow, depending on the underlying system architechture, and so
// its result should be cached if possible.
func (mx DRWMutex) RLocker() sync.Locker {
	return mx[cpus[cpu()]].RLocker()
}

// RLock takes out a non-exclusive reader lock, and returns the lock that was
// taken so that it can later be released.
func (mx DRWMutex) RLock() (l sync.Locker) {
	l = mx[cpus[cpu()]].RLocker()
	l.Lock()
	return
}
