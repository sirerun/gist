//go:build linux

package main

import "syscall"

func peakRSS() (int64, error) {
	var usage syscall.Rusage
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
	return usage.Maxrss * 1024, err
}
