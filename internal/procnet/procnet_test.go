package procnet

import (
	"net"
	"os"
	"testing"
)

func TestOwnersFindsOwnListener(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	got, ok := Owners([]int{port})[port]
	if !ok || got.PID != os.Getpid() {
		t.Errorf("Owners = %+v", got)
	}
	wd, _ := os.Getwd()
	if got.Dir != wd {
		t.Errorf("Dir = %q, erwartet %q", got.Dir, wd)
	}
}
