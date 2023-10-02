// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0
package util

import (
	"os/user"
)

const (
	// Base64Suffix is appended to a label name in the request URL path to
	// mark the following label value as base64 encoded.
	Base64Suffix = "@base64"
)

func IsRoot() (bool, error) {
	u, err := user.Current()
	if err != nil {
		return false, err
	}

	return u.Username == "root", nil
}
