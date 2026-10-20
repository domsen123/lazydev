package docker

import (
	"slices"
	"testing"
)

func TestHostPorts(t *testing.T) {
	tests := map[string][]int{
		"0.0.0.0:5435->5432/tcp, [::]:5435->5432/tcp":                         {5435},
		"0.0.0.0:8025->8025/tcp, [::]:8025->8025/tcp, 0.0.0.0:1025->1025/tcp": {8025, 1025},
		"5432/tcp": nil,
		"":         nil,
	}
	for in, want := range tests {
		if got := (Container{Ports: in}).HostPorts(); !slices.Equal(got, want) {
			t.Errorf("HostPorts(%q) = %v, erwartet %v", in, got, want)
		}
	}
}

func TestLabelsAndProblem(t *testing.T) {
	list, err := parseList(`{"Names":"app-db-1","State":"running","HealthStatus":"unhealthy","Labels":"com.docker.compose.project.config_files=/a.yml,/b.yml,com.docker.compose.project.working_dir=/home/x/app,com.docker.compose.service=db"}
{"Names":"solo","State":"exited","Status":"Exited (0) 2 days ago"}
{"Names":"bad","State":"exited","Status":"Exited (1) 2 days ago"}`)
	if err != nil || len(list) != 3 {
		t.Fatalf("parseList = %v, %v", list, err)
	}
	if list[0].Label(LabelWorkingDir) != "/home/x/app" || list[0].Service() != "db" || !list[0].Problem() {
		t.Errorf("Compose-Container: %+v", list[0])
	}
	if list[1].Service() != "solo" || list[1].Problem() {
		t.Errorf("sauber beendet: %+v", list[1])
	}
	if !list[2].Problem() {
		t.Error("Exit 1 ist kein Problem")
	}
}
