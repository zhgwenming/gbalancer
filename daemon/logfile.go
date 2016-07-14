// Copyright 2014. All rights reserved.
// Use of this source code is governed by a GPLv3
// Author: Wenming Zhang <zhgwenming@gmail.com>

package nestor

import (
	"fmt"
	"io"
	"os"
)

type logFile struct {
	path   string
	file   *os.File
	offset int64
}

func (l *logFile) Open() error {
	file, err := os.OpenFile(l.path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	l.file = file

	// mark the end offset
	offset, err := file.Seek(0, os.SEEK_END)
	if err == nil {
		l.offset = offset
	}

	return err
}

func (l *logFile) Dump(output io.Writer) error {
	buf := make([]byte, 2048)
	_, err := l.file.ReadAt(buf, l.offset)

	fmt.Fprintln(output, "daemon output:")

	fmt.Fprintln(output, "```")
	fmt.Fprintf(output, "%s", buf)
	if err != io.EOF {
		fmt.Println("  ...")
		fmt.Println("```")
		fmt.Println("- Ignored the exceeding contents, please check the output file.")
	} else {
		fmt.Println("```")
	}

	return nil
}
