// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0
package cgroup

import (
	"bytes"
	"fmt"

	"github.com/jasonyangshadow/apptheus/internal/cgroup/parser"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/manager"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const gateway = "metric_gateway"

type CGroup struct {
	cgroups.Manager
}

func NewCGroup(path string) (*CGroup, error) {
	cg := &configs.Cgroup{}
	cg.Path = fmt.Sprintf("/%s/%s", gateway, path)
	mgr, err := manager.New(cg)
	if err != nil {
		return nil, err
	}
	return &CGroup{Manager: mgr}, nil
}

func (c *CGroup) HasProcess() (bool, error) {
	pids, err := c.GetPids()
	return len(pids) != 0, err
}

func (c *CGroup) CreateStats() ([]parser.StatFunc, error) {
	stats, err := c.Manager.GetStats()
	if err != nil {
		return nil, err
	}

	return []parser.StatFunc{
		func() (string, uint64) {
			return "cpu_usage", stats.CpuStats.CpuUsage.TotalUsage
		},
		func() (string, uint64) {
			return "memory_usage", stats.MemoryStats.Usage.Usage
		},
		func() (string, uint64) {
			return "memory_swap_usage", stats.MemoryStats.SwapUsage.Usage
		},
		func() (string, uint64) {
			return "memory_kernel_usage", stats.MemoryStats.KernelUsage.Usage
		},
		func() (string, uint64) {
			return "pid_usage", stats.PidsStats.Current
		},
	}, nil
}

func (c *CGroup) Marshal(buffer *bytes.Buffer) (*bytes.Buffer, error) {
	stats, err := c.CreateStats()
	if err != nil {
		return nil, err
	}

	// write stats
	for _, stat := range stats {
		key, val := stat()
		fmt.Fprintf(buffer, "%s %d\n", key, val)
	}

	return buffer, nil
}
