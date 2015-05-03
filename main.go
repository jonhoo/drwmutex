package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

func cpu() uint64 // implemented in cpu_amd64.s

var cpus map[uint64]int

// determine mapping from APIC ID to CPU index by pinning the entire process to
// one core at the time, and seeing that its APIC ID is.
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
			fmt.Println("cpu", n, "==", oldn, "-- both have CPUID", c)
		}

		cpus[c] = n
		mask <<= 1
		n++
	}

	fmt.Printf("%d/%d cpus found in %v: %v\n", len(cpus), runtime.NumCPU(), time.Now().Sub(start), cpus)

	ret, _, err := syscall.Syscall(syscall.SYS_SCHED_SETAFFINITY, uintptr(0), unsafe.Sizeof(aff), uintptr(unsafe.Pointer(&aff)))
	if ret != 0 {
		panic(err.Error())
	}
}

type RWMutex2 []sync.RWMutex

func (mx RWMutex2) Lock() {
	for core := range mx {
		mx[core].Lock()
	}
}

func (mx RWMutex2) Unlock() {
	for core := range mx {
		mx[core].Unlock()
	}
}

func main() {
	cpuprofile := flag.Bool("cpuprofile", false, "enable CPU profiling")
	locks := flag.Uint64("i", 10000, "Number of iterations to perform")
	write := flag.Float64("p", 0.0001, "Probability of write locks")
	wwork := flag.Int("w", 1, "Amount of work for each writer")
	rwork := flag.Int("r", 100, "Amount of work for each reader")
	readers := flag.Int("n", runtime.GOMAXPROCS(0), "Total number of readers")
	checkcpu := flag.Uint64("c", 100, "Update CPU estimate every n iterations")
	flag.Parse()

	var o *os.File
	if *cpuprofile {
		o, _ := os.Create("rw.out")
		pprof.StartCPUProfile(o)
	}

	readers_per_core := *readers / runtime.GOMAXPROCS(0)

	var wg sync.WaitGroup

	var mx1 sync.RWMutex

	start1 := time.Now()
	for n := 0; n < runtime.GOMAXPROCS(0); n++ {
		for r := 0; r < readers_per_core; r++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r := rand.New(rand.NewSource(rand.Int63()))
				for n := uint64(0); n < *locks; n++ {
					if r.Float64() < *write {
						mx1.Lock()
						x := 0
						for i := 0; i < *wwork; i++ {
							x++
						}
						_ = x
						mx1.Unlock()
					} else {
						mx1.RLock()
						x := 0
						for i := 0; i < *rwork; i++ {
							x++
						}
						_ = x
						mx1.RUnlock()
					}
				}
			}()
		}
	}
	wg.Wait()
	end1 := time.Now()

	t1 := end1.Sub(start1)
	fmt.Println("mx1", runtime.GOMAXPROCS(0), *readers, *locks, *write, *wwork, *rwork, *checkcpu, t1.Seconds(), t1)

	if *cpuprofile {
		pprof.StopCPUProfile()
		o.Close()

		o, _ = os.Create("rw2.out")
		pprof.StartCPUProfile(o)
	}

	mx2 := make(RWMutex2, len(cpus))

	start2 := time.Now()
	for n := 0; n < runtime.GOMAXPROCS(0); n++ {
		for r := 0; r < readers_per_core; r++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c := cpus[cpu()]
				r := rand.New(rand.NewSource(rand.Int63()))
				for n := uint64(0); n < *locks; n++ {
					if *checkcpu != 0 && n%*checkcpu == 0 {
						c = cpus[cpu()]
					}

					if r.Float64() < *write {
						mx2.Lock()
						x := 0
						for i := 0; i < *wwork; i++ {
							x++
						}
						_ = x
						mx2.Unlock()
					} else {
						mx2[c].RLock()
						x := 0
						for i := 0; i < *rwork; i++ {
							x++
						}
						_ = x
						mx2[c].RUnlock()
					}
				}
			}()
		}
	}
	wg.Wait()
	end2 := time.Now()

	pprof.StopCPUProfile()
	o.Close()

	t2 := end2.Sub(start2)
	fmt.Println("mx2", runtime.GOMAXPROCS(0), *readers, *locks, *write, *wwork, *rwork, *checkcpu, t2.Seconds(), t2)
}
