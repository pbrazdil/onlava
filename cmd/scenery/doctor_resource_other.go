//go:build !unix && !windows

package main

import "context"

func (defaultDoctorResourceProbe) Disk(context.Context, string) (doctorDiskInfo, error) {
	return doctorDiskInfo{}, errDoctorResourceUnsupported
}

func (defaultDoctorResourceProbe) Memory(context.Context) (doctorMemoryInfo, error) {
	return doctorMemoryInfo{}, errDoctorResourceUnsupported
}
