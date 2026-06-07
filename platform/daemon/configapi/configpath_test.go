package configapi

import (
	"net/http/httptest"
	"testing"
)

func TestConfigPath_ConfinesToWorkspace(t *testing.T) {
	cases := []struct {
		query   string
		want    string
		wantErr bool
	}{
		{"", "runfabric.yml", false},
		{"runfabric.yml", "runfabric.yml", false},
		{"subdir/runfabric.yml", "subdir/runfabric.yml", false},
		{"../../other/runfabric.yml", "", true},
		{"/etc/passwd", "", true},
		{"../secret.yml", "", true},
	}
	for _, c := range cases {
		target := "/resolve"
		if c.query != "" {
			target += "?config=" + c.query
		}
		req := httptest.NewRequest("POST", target, nil)
		got, err := configPath(req)
		if (err != nil) != c.wantErr {
			t.Errorf("configPath(config=%q) err=%v, wantErr=%v", c.query, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("configPath(config=%q) = %q, want %q", c.query, got, c.want)
		}
	}
}
