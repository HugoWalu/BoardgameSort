package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultPort   = 8080
	defaultAppURL = "http://localhost:8080"
)

type Config struct {
	Port        int
	AppOrigin   string
	DatabaseURL string
	AuthSecret  string
}

func envVariable(env map[string]string, variable string) (string, error) {
	value := env[variable]
	if value == "" {
		return value, errors.New("Variable not found or empty")
	}
	return value, nil
}

func Load(workingDirectory string) (Config, error) {
	env, err := LoadEnv(workingDirectory)
	if err != nil {
		return Config{}, err
	}
	return Parse(env)
}

func Parse(environment map[string]string) (Config, error) {
	port, err := parseNonNegativeInteger(valueOrDefault(environment, "PORT", strconv.Itoa(defaultPort)), "PORT")
	if err != nil {
		return Config{}, err
	}
	if port > 65_535 {
		return Config{}, errors.New("PORT must be no greater than 65535")
	}
	appOrigin, err := parseOrigin(valueOrDefault(environment, "APP_URL", defaultAppURL))
	if err != nil {
		return Config{}, err
	}

	authSecret := environment["AUTH_SECRET"]
	if authSecret == "" || authSecret == "Placeholder" {
		return Config{}, errors.New("AUTH_SECRET must be set")
	}

	databaseURL := environment["DATABASE_URL"]
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL must be set")
	}

	return Config{
		Port:        port,
		AppOrigin:   appOrigin,
		DatabaseURL: databaseURL,
		AuthSecret:  authSecret,
	}, nil
}

func parseOrigin(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("APP_URL must be an absolute URL")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func valueOrDefault(environment map[string]string, name, fallback string) string {
	if value := environment[name]; value != "" {
		return value
	}
	return fallback
}

func parseNonNegativeInteger(value, name string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return parsed, nil
}

func LoadEnv(workingDirectory string) (map[string]string, error) {
	env := make(map[string]string)
	envPath := filepath.Join(workingDirectory, ".env.local")
	envRead, err := godotenv.Read(envPath)
	if !os.IsNotExist(err) && err != nil {
		return env, err
	}
	if err == nil {
		env = envRead
	}
	envOs := os.Environ()
	for _, envvar := range envOs {
		split := strings.SplitN(envvar, "=", 2)
		env[split[0]] = split[1]
	}
	return env, nil
}
