package cli

import "testing"

func TestParseScaleFlags(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected map[string]int
		wantErr  bool
	}{
		{"empty", nil, nil, false},
		{"single", []string{"web=3"}, map[string]int{"web": 3}, false},
		{"multiple", []string{"web=3", "worker=2"}, map[string]int{"web": 3, "worker": 2}, false},
		{"one replica", []string{"db=1"}, map[string]int{"db": 1}, false},
		{"invalid format", []string{"web"}, nil, true},
		{"zero replicas", []string{"web=0"}, nil, true},
		{"negative replicas", []string{"web=-1"}, nil, true},
		{"not a number", []string{"web=abc"}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseScaleFlags(tt.flags)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseScaleFlags(%v) error = %v, wantErr %v", tt.flags, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if tt.expected == nil && got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				for k, v := range tt.expected {
					if got[k] != v {
						t.Errorf("expected %s=%d, got %d", k, v, got[k])
					}
				}
			}
		})
	}
}
