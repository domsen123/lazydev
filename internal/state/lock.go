package state

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// MergeLock is the name of the lock that guards the base branch of project id during merges, pulls
// and pushes.
func MergeLock(id string) string {
	return id + ".merge"
}

// Lock locks locks/<name>.lock with flock until unlock runs, waiting while another process holds it.
func Lock(name string) (unlock func(), err error) {
	return flock(name, syscall.LOCK_EX)
}

// TryLock locks like Lock without waiting. unlock is nil if another process holds the lock.
func TryLock(name string) (unlock func(), err error) {
	unlock, err = flock(name, syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return nil, nil
	}
	return unlock, err
}

func flock(name string, how int) (func(), error) {
	path := filepath.Join(Dir(), "locks", name+".lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
