package driver

import "testing"

func TestExtractServiceFromName(t *testing.T) {
	tests := []struct {
		name        string
		container   string
		project     string
		expected    string
	}{
		{"simple", "myapp-web-1", "myapp", "web"},
		{"hyphenated service", "myapp-my-service-1", "myapp", "my-service"},
		{"multi-hyphen", "myapp-my-cool-api-2", "myapp", "my-cool-api"},
		{"no match prefix", "other-web-1", "myapp", "other-web"},
		{"single part", "myapp-web", "myapp", "web"},
		{"replica number", "proj-db-3", "proj", "db"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractServiceFromName(tt.container, tt.project)
			if got != tt.expected {
				t.Errorf("extractServiceFromName(%q, %q) = %q, want %q", tt.container, tt.project, got, tt.expected)
			}
		})
	}
}

func TestIntFromJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected int
	}{
		{"float64", float64(8080), 8080},
		{"int", int(3000), 3000},
		{"string", "not a number", 0},
		{"nil", nil, 0},
		{"zero float", float64(0), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intFromJSON(tt.input)
			if got != tt.expected {
				t.Errorf("intFromJSON(%v) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatPublishedPorts(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]any
		expected string
	}{
		{
			"no ports",
			map[string]any{},
			"",
		},
		{
			"single port",
			map[string]any{
				"publishedPorts": []any{
					map[string]any{
						"hostAddress":   "0.0.0.0",
						"hostPort":      float64(8080),
						"containerPort": float64(80),
						"proto":         "tcp",
					},
				},
			},
			"0.0.0.0:8080->80/tcp",
		},
		{
			"default host address and proto",
			map[string]any{
				"publishedPorts": []any{
					map[string]any{
						"hostPort":      float64(3000),
						"containerPort": float64(3000),
					},
				},
			},
			"0.0.0.0:3000->3000/tcp",
		},
		{
			"multiple ports",
			map[string]any{
				"publishedPorts": []any{
					map[string]any{
						"hostPort":      float64(8080),
						"containerPort": float64(80),
						"proto":         "tcp",
					},
					map[string]any{
						"hostPort":      float64(443),
						"containerPort": float64(443),
						"proto":         "tcp",
					},
				},
			},
			"0.0.0.0:8080->80/tcp, 0.0.0.0:443->443/tcp",
		},
		{
			"udp protocol",
			map[string]any{
				"publishedPorts": []any{
					map[string]any{
						"hostPort":      float64(53),
						"containerPort": float64(53),
						"proto":         "udp",
					},
				},
			},
			"0.0.0.0:53->53/udp",
		},
		{
			"skip invalid entry",
			map[string]any{
				"publishedPorts": []any{
					"not a map",
					map[string]any{
						"hostPort":      float64(80),
						"containerPort": float64(80),
					},
				},
			},
			"0.0.0.0:80->80/tcp",
		},
		{
			"skip zero ports",
			map[string]any{
				"publishedPorts": []any{
					map[string]any{
						"hostPort":      float64(0),
						"containerPort": float64(80),
					},
				},
			},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPublishedPorts(tt.config)
			if got != tt.expected {
				t.Errorf("formatPublishedPorts() = %q, want %q", got, tt.expected)
			}
		})
	}
}
