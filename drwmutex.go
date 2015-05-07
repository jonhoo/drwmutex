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

func cpu() uint64 // implemented in cpu_*.s

var cpus map[uint64]int

func init() {
	cpus = make(map[uint64]int)

	var aff uint64
	syscall.Syscall(syscall.SYS_SCHED_GETAFFINITY, uintptr(0), unsafe.Sizeof(aff), uintptr(unsafe.Pointer(&aff)))

	n := 0
	start := time.Now()
	var mask uint64 = 1
Outer:
	for {
		for (aff & mask) == 0 {
			mask <<= 1
			if mask == 0 || mask > aff {
				break Outer
			}
		}

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

	ret, _, err := syscall.Syscall(syscall.SYS_SCHED_SETAFFINITY, uintptr(0), unsafe.Sizeof(aff), uintptr(unsafe.Pointer(&aff)))
	if ret != 0 {
		panic(err.Error())
	}
}

type DRWMutex []sync.RWMutex

func New() DRWMutex {
	return make(DRWMutex, runtime.GOMAXPROCS(0))
}

func (mx DRWMutex) Lock() {
	for core := range mx {
		mx[core].Lock()
	}
}

func (mx DRWMutex) Unlock() {
	for core := range mx {
		mx[core].Unlock()
	}
}

func (mx DRWMutex) RLocker() sync.Locker {
	return mx[cpus[cpu()]].RLocker()
}

func (mx DRWMutex) RLock() (l sync.Locker) {
	l = mx[cpus[cpu()]].RLocker()
	l.Lock()
	return
}
