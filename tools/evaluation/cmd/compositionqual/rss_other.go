//go:build !linux

package main

import "errors"

func peakRSS() (int64, error) {
	return 0, errors.New("peak RSS measurement requires frozen Linux target")
}
