// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

// +build go1.4

package nestor

import (
	"os"
)

const (
	UNSET_ENV = false // set it to false to support reexec
)

func unsetenv(env string) error {
	if UNSET_ENV {
		return os.Unsetenv(env)
	} else {
		return nil
	}
}
