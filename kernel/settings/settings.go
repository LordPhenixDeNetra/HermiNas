// Package settings loads HermiNas.yaml once at boot into an immutable
// struct (leçon aNtaerus: config immuable, 1 seule source). Fields are
// unexported and reachable only through getters, so nothing downstream can
// mutate configuration at runtime.
package settings

import (
	"fmt"
	"os"

	"github.com/spf13/viper"

	"herminas/kernel/errors"
)

type rawHTTP struct {
	Port int `mapstructure:"port"`
}

type rawGRPC struct {
	Port int `mapstructure:"port"`
}

type rawLLM struct {
	Provider string `mapstructure:"provider"`
	APIKey   string `mapstructure:"api_key"`
}

type rawSettings struct {
	Environment string  `mapstructure:"environment"`
	HTTP        rawHTTP `mapstructure:"http"`
	GRPC        rawGRPC `mapstructure:"grpc"`
	DataDir     string  `mapstructure:"data_dir"`
	LLM         rawLLM  `mapstructure:"llm"`
}

type Settings struct {
	environment string
	httpPort    int
	grpcPort    int
	dataDir     string
	llmProvider string
	llmAPIKey   SecretString
}

// Load reads path (YAML) plus HERMINAS_-prefixed environment overrides and
// returns an immutable Settings. Call it exactly once at boot.
func Load(path string) (*Settings, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("HERMINAS")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, errors.Wrap(errors.CodeNotFound, fmt.Sprintf("cannot read settings file %q", path), err)
	}

	var raw rawSettings
	if err := v.Unmarshal(&raw); err != nil {
		return nil, errors.Wrap(errors.CodeInvalidArgument, "invalid settings file", err)
	}

	apiKey := raw.LLM.APIKey
	if envKey := os.Getenv("HERMINAS_LLM_API_KEY"); envKey != "" {
		apiKey = envKey
	}

	return &Settings{
		environment: raw.Environment,
		httpPort:    raw.HTTP.Port,
		grpcPort:    raw.GRPC.Port,
		dataDir:     raw.DataDir,
		llmProvider: raw.LLM.Provider,
		llmAPIKey:   NewSecretString(apiKey),
	}, nil
}

func (s *Settings) Environment() string     { return s.environment }
func (s *Settings) HTTPPort() int           { return s.httpPort }
func (s *Settings) GRPCPort() int           { return s.grpcPort }
func (s *Settings) DataDir() string         { return s.dataDir }
func (s *Settings) LLMProvider() string     { return s.llmProvider }
func (s *Settings) LLMAPIKey() SecretString { return s.llmAPIKey }
