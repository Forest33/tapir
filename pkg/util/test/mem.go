package test

import (
	"fmt"
	"runtime"
)

func MemUsage() runtime.MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Alloc = %v MiB", m.Alloc/1024/1024)
	fmt.Printf("\tTotalAlloc = %v MiB", m.TotalAlloc/1024/1024)
	fmt.Printf("\tSys = %v MiB", m.Sys/1024/1024)
	fmt.Printf("\tNumGC = %v", m.NumGC)
	fmt.Printf("\tMallocs = %v", m.Mallocs)
	fmt.Printf("\tFrees = %v\n", m.Frees)
	return m
}
