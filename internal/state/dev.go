package state

import "path/filepath"

// DevPIDPath holds PID and folder of the dev server of a PRD. It is a file of its own, because
// the runner overwrites the run state all the time.
func DevPIDPath(id, prdName string) string {
	return filepath.Join(RunsDir(id), prdName+".devpid")
}

// DevLogPath takes stdout and stderr of the dev server of a PRD.
func DevLogPath(id, prdName string) string {
	return filepath.Join(RunsDir(id), prdName+".dev.log")
}
