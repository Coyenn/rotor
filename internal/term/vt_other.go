//go:build !windows

package term

import "io"

// enableWindowsVT is a no-op on platforms whose terminals already interpret
// ANSI sequences natively.
func enableWindowsVT(io.Writer) {}
