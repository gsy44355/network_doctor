package target

import (
	"testing"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		input  string
		scheme string
		host   string
		port   int
		isIP   bool
	}{
		{"https://example.com", "https", "example.com", 443, false},
		{"http://example.com", "http", "example.com", 80, false},
		{"http://example.com:8080", "http", "example.com", 8080, false},
		{"mysql://db.host:3306", "mysql", "db.host", 3306, false},
		{"redis://cache:6379", "redis", "cache", 6379, false},
		{"example.com:5432", "postgresql", "example.com", 5432, false},
		{"example.com:80", "http", "example.com", 80, false},
		{"example.com:3306", "mysql", "example.com", 3306, false},
		{"example.com:6379", "redis", "example.com", 6379, false},
		{"example.com:22", "ssh", "example.com", 22, false},
		{"example.com:8080", "tcp", "example.com", 8080, false},
		{"example.com", "https", "example.com", 443, false},
		{"1.2.3.4:3306", "mysql", "", 3306, true},
		{"1.2.3.4", "https", "", 443, true},
		{"mysql://user:pass@db.host:3306", "mysql", "db.host", 3306, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			target, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			if target.Scheme != tt.scheme {
				t.Errorf("scheme = %q, want %q", target.Scheme, tt.scheme)
			}
			if target.Host != tt.host {
				t.Errorf("host = %q, want %q", target.Host, tt.host)
			}
			if target.Port != tt.port {
				t.Errorf("port = %d, want %d", target.Port, tt.port)
			}
			if target.IsIP != tt.isIP {
				t.Errorf("isIP = %v, want %v", target.IsIP, tt.isIP)
			}
		})
	}
}

func TestParseTargetFile(t *testing.T) {
	content := "# comment line\nhttps://api.example.com\n\nmysql://db.internal:3306\n# another comment\nredis://cache:6379\n"
	targets, err := ParseLines(content)
	if err != nil {
		t.Fatalf("ParseLines error: %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("got %d targets, want 3", len(targets))
	}
	if targets[0].Scheme != "https" {
		t.Errorf("targets[0].Scheme = %q, want https", targets[0].Scheme)
	}
	if targets[1].Scheme != "mysql" {
		t.Errorf("targets[1].Scheme = %q, want mysql", targets[1].Scheme)
	}
	if targets[2].Scheme != "redis" {
		t.Errorf("targets[2].Scheme = %q, want redis", targets[2].Scheme)
	}
}
