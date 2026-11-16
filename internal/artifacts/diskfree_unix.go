//go:build unix

package artifacts

import "golang.org/x/sys/unix"

// diskFree is the space available to this user on the disk holding dir, or
// on its nearest existing parent.
func diskFree(dir string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(existingParent(dir), &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}
