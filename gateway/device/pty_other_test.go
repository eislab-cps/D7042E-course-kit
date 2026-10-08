//go:build !linux

package device

import (
	"errors"
	"os"
)

func openPTY() (*os.File, string, error) { return nil, "", errors.New("pty test is Linux-only") }
func makeRaw(*os.File) error             { return errors.New("pty test is Linux-only") }
