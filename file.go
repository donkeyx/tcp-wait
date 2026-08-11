package tcpwait

import (
	"fmt"
	"os"
)

// probeFile succeeds when path exists (file, socket, dir, symlink target, etc.).
func probeFile(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", path)
	}
	return err
}
