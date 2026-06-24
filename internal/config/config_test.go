package config

import "testing"

func TestConfigRecordIsLocal(t *testing.T) {
	cases := []struct {
		name string
		rec  ConfigRecord
		want bool
	}{
		{"explicit local", ConfigRecord{Type: ConfigTypeLocal}, true},
		{"explicit remote", ConfigRecord{Type: ConfigTypeRemote}, false},
		{"legacy remote with URL", ConfigRecord{URL: "http://example.com"}, false},
		{"legacy local empty", ConfigRecord{}, true},
		{"remote type with empty URL", ConfigRecord{Type: ConfigTypeRemote, URL: ""}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.rec.IsLocal(); got != c.want {
				t.Fatalf("IsLocal() = %v, want %v", got, c.want)
			}
		})
	}
}
