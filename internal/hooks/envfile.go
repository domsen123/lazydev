package hooks

import (
	"bufio"
	"strings"
)

// parseEnv liest KEY=VALUE-Zeilen einer .env-Datei. Kommentare und "export " überspringt es,
// Anführungszeichen um den Wert entfernt es.
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
		val = val[1 : len(val)-1]
	}
	return key, val, true
}

// setEnv ersetzt jede Zeile mit key durch key="value". Fehlt key, hängt setEnv die Zeile an.
// Alle anderen Zeilen bleiben unverändert.
func setEnv(data, key, value string) string {
	line := key + `="` + value + `"`
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
