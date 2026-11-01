// Package core is a fixture package.
package core

import "os"

// Home returns the home directory.
func Home() string {
	return os.Getenv("HOME")
}
