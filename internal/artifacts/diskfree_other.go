//go:build !unix && !windows

package artifacts

import "errors"

// diskFree is unknown here; min_free is then not checked.
func diskFree(string) (uint64, error) { return 0, errors.New("free space unknown on this system") }
