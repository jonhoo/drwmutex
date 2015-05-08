package drwmutex

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func map_cpus() (cpus map[uint64]int) {
	cpus = make(map[uint64]int)

	var aff uint64
	syscall.Syscall(syscall.SYS_SCHED_GETAFFINITY, uintptr(0), unsafe.Sizeof(aff), uintptr(unsafe.Pointer(&aff)))

	n := 0
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

	// give process access to all the cores it had access to initially
	ret, _, err := syscall.Syscall(syscall.SYS_SCHED_SETAFFINITY, uintptr(0), unsafe.Sizeof(aff), uintptr(unsafe.Pointer(&aff)))
	if ret != 0 {
		panic(err.Error())
	}

	return
}
