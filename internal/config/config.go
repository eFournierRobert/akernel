package config

import (
	"io"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
)

type Model struct {
	Nickname    string
	Id          string
	Description string
}

type TomlConfig struct {
	Ollama struct {
		Models []Model
	}
}

type Config struct {
	SystemPrompt string
	OllamaModels []Model
}

func ReadConfig() (Config, error) {
	sysPrompt := readSystemPrompt()
	tomlConfig, err := readTomlConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		SystemPrompt: sysPrompt,
		OllamaModels: tomlConfig.Ollama.Models,
	}, nil
}

func readTomlConfig() (TomlConfig, error) {
	tomlConfigPath, err := xdg.ConfigFile("akernel/config.toml")

	var config TomlConfig
	_, err = toml.DecodeFile(tomlConfigPath, &config)
	return config, err
}

func readSystemPrompt() string {
	systemPromptPath, err := xdg.ConfigFile("akernel/system.md")
	if err != nil {
		return ""
	}

	f, err := os.Open(systemPromptPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	systemPrompt, err := io.ReadAll(f)
	if err != nil {
		return ""
	}

	return string(systemPrompt)
}
