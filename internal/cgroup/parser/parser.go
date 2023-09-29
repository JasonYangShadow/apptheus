// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package parser

import "bytes"

type Marshal interface {
	Marshal(*bytes.Buffer) (*bytes.Buffer, error)
}

type StatFunc func() (string, uint64)

type Stat interface {
	CreateStats() ([]StatFunc, error)
}

type ContainerInfo struct {
	FullPath string
	Pid      uint64
	Exe      string
	Id       string
}
