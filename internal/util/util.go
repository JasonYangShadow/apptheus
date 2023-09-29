// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0
package util

import (
	"encoding/base64"
	"fmt"
	"os/user"
	"strings"

	"github.com/prometheus/common/model"
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

// decodeBase64 decodes the provided string using the “Base 64 Encoding with URL
// and Filename Safe Alphabet” (RFC 4648). Padding characters (i.e. trailing
// '=') are ignored.
func DecodeBase64(s string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	return string(b), err
}

// splitLabels splits a labels string into a label map mapping names to values.
func SplitLabels(labels string) (map[string]string, error) {
	result := map[string]string{}
	if len(labels) <= 1 {
		return result, nil
	}
	components := strings.Split(labels[1:], "/")
	if len(components)%2 != 0 {
		return nil, fmt.Errorf("odd number of components in label string %q", labels)
	}

	for i := 0; i < len(components)-1; i += 2 {
		name, value := components[i], components[i+1]
		trimmedName := strings.TrimSuffix(name, Base64Suffix)
		if !model.LabelNameRE.MatchString(trimmedName) ||
			strings.HasPrefix(trimmedName, model.ReservedLabelPrefix) {
			return nil, fmt.Errorf("improper label name %q", trimmedName)
		}
		if name == trimmedName {
			result[name] = value
			continue
		}
		decodedValue, err := DecodeBase64(value)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 encoding for label %s=%q: %w", trimmedName, value, err)
		}
		result[trimmedName] = decodedValue
	}
	return result, nil
}
