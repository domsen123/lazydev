package database

import (
	"bufio"
	"errors"
	"net/url"
	"slices"
	"strings"
)

// parseEnv reads the KEY=VALUE lines of an env file. It skips comments and "export ", and removes
// quotes around the value.
func parseEnv(data string) map[string]string {
	vals := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(data))
	for sc.Scan() {
		key, val, ok := splitEnvLine(sc.Text())
		if ok {
			vals[key] = val
		}
	}
	return vals
}

// Keys lists the keys of an env file in file order, each once.
func Keys(data string) []string {
	var keys []string
	sc := bufio.NewScanner(strings.NewReader(data))
	for sc.Scan() {
		if key, _, ok := splitEnvLine(sc.Text()); ok && key != "" && !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	return keys
}

func splitEnvLine(line string) (key, val string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	key, val, ok = strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	val = strings.TrimSpace(val)
	if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
		quote := val[0]
		val = val[1 : len(val)-1]
		if quote == '"' {
			val = strings.ReplaceAll(val, `\"`, `"`)
		}
	}
	return key, val, true
}

// setEnv replaces every line with key by key="value", a " in value escaped as \". Without key,
// setEnv appends the line. All other lines stay unchanged.
func setEnv(data, key, value string) string {
	line := key + `="` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	lines := strings.Split(data, "\n")
	found := false
	for i, l := range lines {
		if k, _, ok := splitEnvLine(l); ok && k == key {
			lines[i] = line
			found = true
		}
	}
	out := strings.Join(lines, "\n")
	if found {
		return out
	}
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out + line + "\n"
}

// errNoName means a URL without a database name in its path.
var errNoName = errors.New("no database name")

// isURL is true for a value like "postgres://…".
func isURL(value string) bool {
	return strings.Contains(value, "://")
}

// BaseName is the database name in value: the path of a URL without the leading "/", otherwise
// the whole value.
func BaseName(value string) (string, error) {
	if !isURL(value) {
		return value, nil
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return "", errNoName
	}
	return name, nil
}

// ReplaceName sets the database name in value. In a URL only the path changes, scheme, user,
// password, host, port and query stay as they are.
func ReplaceName(value, name string) string {
	if !isURL(value) {
		return name
	}
	scheme := strings.Index(value, "://") + len("://")
	rest := value[scheme:]
	start := strings.IndexAny(rest, "/?#")
	if start < 0 {
		return value + "/" + url.PathEscape(name)
	}
	tail := rest[start:]
	end := len(tail)
	if tail[0] == '/' {
		if i := strings.IndexAny(tail, "?#"); i >= 0 {
			end = i
		}
	} else {
		end = 0
	}
	return value[:scheme+start] + "/" + url.PathEscape(name) + tail[end:]
}
