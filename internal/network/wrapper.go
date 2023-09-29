// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/jasonyangshadow/apptheus/internal/cgroup/parser"
	"github.com/jasonyangshadow/apptheus/internal/monitor"
	"github.com/jasonyangshadow/apptheus/storage"
	"github.com/prometheus/exporter-toolkit/web"
	"toolman.org/net/peercred"
)

type ServerOption struct {
	Server      *http.Server
	WebConfig   *web.FlagConfig
	MetricStore storage.MetricStore
	Logger      log.Logger
	SocketPath  string
	TrustedPath string
	Interval    *time.Ticker
	ErrCh       chan error
}

type WrappedInstance struct {
	*parser.ContainerInfo
	*monitor.MonitorInstance
	net.Conn
}

type WrappedListener struct {
	*peercred.Listener
	TrustedPath  string
	Option       *ServerOption
	ContainerMap map[string]*WrappedInstance
}

func (l *WrappedListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	pid := conn.(*peercred.Conn).Ucred.Pid

	// verification by pid
	link, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return nil, err
	}

	exe := filepath.Base(link)
	verify := false
	for _, path := range strings.Split(l.TrustedPath, ";") {
		if strings.TrimSpace(link) == strings.TrimSpace(path) {
			verify = true
		}
	}

	if !verify {
		if conn != nil {
			conn.Close()
		}
		level.Error(l.Option.Logger).Log("msg", fmt.Sprintf("%s is not trusted, connection rejected", link))
		return conn, nil
	}

	// container and monitor instance info
	container := &parser.ContainerInfo{
		FullPath: link,
		Pid:      uint64(pid),
		Exe:      exe,
		Id:       fmt.Sprintf("%s_%d", exe, pid),
	}
	instance := monitor.New(l.Option.Interval)

	// save the container info for further usage
	l.ContainerMap[container.Id] = &WrappedInstance{
		ContainerInfo:   container,
		MonitorInstance: instance,
		Conn:            conn,
	}

	// fire monitor thread
	go instance.Start(container, l.Option.MetricStore, l.Option.Logger)

	level.Info(l.Option.Logger).Log("msg", "New connection established", "container id", container.Id, "container pid", container.Pid, "container full path", container.FullPath)

	return conn, nil
}
