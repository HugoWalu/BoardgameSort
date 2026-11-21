package config

import "testing"

func validEnvironment() map[string]string {
	return map[string]string{
		"PORT":         "9090",
		"APP_URL":      "https://example.com/some/path",
		"AUTH_SECRET":  "test-secret",
		"DATABASE_URL": "postgres://app:app@localhost:5432/boardgames?sslmode=disable",
	}
}

func TestParseValid(t *testing.T) {
	cfg, err := Parse(validEnvironment())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.AppOrigin != "https://example.com" {
		t.Errorf("AppOrigin = %q, want %q", cfg.AppOrigin, "https://example.com")
	}
	if cfg.AuthSecret != "test-secret" {
		t.Errorf("AuthSecret = %q, want %q", cfg.AuthSecret, "test-secret")
	}
	if cfg.DatabaseURL != "postgres://app:app@localhost:5432/boardgames?sslmode=disable" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name        string
		environment map[string]string
		want        string
	}{
		{
			name: "missing DATABASE_URL",
			environment: func() map[string]string {
				env := validEnvironment()
				delete(env, "DATABASE_URL")
				return env
			}(),
			want: "DATABASE_URL must be set",
		},
		{
			name: "missing AUTH_SECRET",
			environment: func() map[string]string {
				env := validEnvironment()
				delete(env, "AUTH_SECRET")
				return env
			}(),
			want: "AUTH_SECRET must be set",
		},
		{
			name: "invalid APP_URL",
			environment: func() map[string]string {
				env := validEnvironment()
				env["APP_URL"] = "not-a-url"
				return env
			}(),
			want: "APP_URL must be an absolute URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.environment)
			if err == nil {
				t.Fatal("Parse returned nil error")
			}
			if err.Error() != tt.want {
				t.Fatalf("error = %q, want %q", err, tt.want)
			}
		})
	}
}
