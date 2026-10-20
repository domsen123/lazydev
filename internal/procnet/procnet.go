// Package procnet findet über /proc den Prozess, der auf einem TCP-Port lauscht.
package procnet

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// tcpListen ist der Zustand LISTEN in /proc/net/tcp.
const tcpListen = "0A"

// Listener ist ein Prozess, der auf einem Port lauscht.
type Listener struct {
	PID  int
	Name string
	Dir  string
}

// Listening liefert die lauschenden TCP-Ports auf IPv4 und IPv6 mit der Socket-Inode.
func Listening() map[int][]string {
	ports := map[int][]string{}
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Scan()
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) < 10 || fields[3] != tcpListen {
				continue
			}
			_, portHex, ok := strings.Cut(fields[1], ":")
			if !ok {
				continue
			}
			port, err := strconv.ParseInt(portHex, 16, 32)
			if err != nil {
				continue
			}
			ports[int(port)] = append(ports[int(port)], fields[9])
		}
		fh.Close()
	}
	return ports
}

// Owners ordnet jedem lauschenden Port aus ports den Prozess zu. Ports fremder Nutzer ohne
// Zugriff auf /proc/<pid>/fd fehlen im Ergebnis.
func Owners(ports []int) map[int]Listener {
	listening := Listening()
	inodes := map[string]int{}
	for _, p := range ports {
		for _, inode := range listening[p] {
			inodes["socket:["+inode+"]"] = p
		}
	}
	out := map[int]Listener{}
	if len(inodes) == 0 {
		return out
	}
	procs, _ := filepath.Glob("/proc/[0-9]*")
	for _, proc := range procs {
		fds, err := os.ReadDir(filepath.Join(proc, "fd"))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(proc, "fd", fd.Name()))
			if err != nil {
				continue
			}
			port, ok := inodes[target]
			if !ok {
				continue
			}
			if _, done := out[port]; done {
				continue
			}
			pid, _ := strconv.Atoi(filepath.Base(proc))
			name, _ := os.ReadFile(filepath.Join(proc, "comm"))
			dir, _ := os.Readlink(filepath.Join(proc, "cwd"))
			out[port] = Listener{PID: pid, Name: strings.TrimSpace(string(name)), Dir: dir}
		}
	}
	return out
}

func (l Listener) String() string {
	return fmt.Sprintf("%s (PID %d)", l.Name, l.PID)
}
