package config

import (
	"fmt"
	"os"
	yaml "gopkg.in/yaml.v3"
)

const (
	configEnvKey = "config"
)

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"server"`
	Monitoring struct {
		StatusURL string `yaml:"status-url"`
	} `yaml:"monitoring"`
	Postgres struct {
		Database string `yaml:"database"`
		Host string `yaml:"host"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		QueriesPath string `yaml:"queries-path"`
	} `yaml:"postgres"`
	NowPayments struct {
		APIKey string `yaml:"api-key"`
		IsSandbox bool `yaml:"is-sandbox"`
		IPNSecretKey string `yaml:"ipn-secret-key"`
		IPNCallbackURL string `yaml:"ipn-callback-url"`
	} `yaml:"now-payments"`
	Cardano struct {
		HotWalletSigningKeyPath string `yaml:"hot-wallet-signing-key-path"`
		HotWalletAddress string `yaml:"hot-wallet-address"`
		ScriptsPath string `yaml:"scripts-path"`
		ProtocolParamsPath string `yaml:"protocol-params-path"`
	} `yaml:"cardano"`
	Mocked bool `yaml:"mocked"`
}

// gets the path of the config YAML from launch args
func GetConfigPath() (string, error) {
	if (len(os.Args) != 2) {
		return "", fmt.Errorf("expected config path as first argument")
	}
	
	configPath := os.Args[1]
	return configPath, nil
}

// loads config YAML and stores in env
func LoadConfig(configPath string) error {
	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)

	var config Config
	err = decoder.Decode(&config)
	if err != nil {
		return err
	}

	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	err = os.Setenv(configEnvKey, string(yamlBytes))
	if err != nil {
		return err
	}

	return nil
}

// checks that the config contains all required fields
func validateConfig(config Config) error {
	if len(config.Monitoring.StatusURL) == 0 {
		return fmt.Errorf("monitoring status URL cannot be empty")
	}
	if len(config.NowPayments.IPNCallbackURL) == 0 {
		return fmt.Errorf("IPN callback URL cannot be empty")
	}
	return nil
}

// gets the config from the env
func GetConfig() (*Config, error) {
	yamlBytes := []byte(os.Getenv(configEnvKey))

	var config Config
	err := yaml.Unmarshal(yamlBytes, &config)
	if err != nil {
		return nil, err
	}

	err = validateConfig(config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
