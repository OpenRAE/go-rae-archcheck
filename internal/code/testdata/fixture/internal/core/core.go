// Package core is a fixture library package.
package core

import "os"

// Name returns the value of an environment variable.
func Name() string {
	return os.Getenv("NAME")
}
