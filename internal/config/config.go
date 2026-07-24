// Package config handles reading application configuration from XDG paths,
// including the TOML config file and the system prompt markdown file.
package config

import (
	"io"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
)

// Model maps a user-facing nickname to an LLM provider's model ID with a description and provider label.
type Model struct {
	Name        string
	Id          string
	Description string
	Provider    string
}

// TomlConfig mirrors the shape of config.toml.
type TomlConfig struct {
	Models []Model
}

// Config bundles all startup configuration consumed by main.
type Config struct {
	SystemPrompt string
	Models       []Model
}

// ReadConfig loads the system prompt and TOML config, returning a ready-to-use Config.
func ReadConfig() (Config, error) {
	sysPrompt := readSystemPrompt()
	tomlConfig, err := readTomlConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		SystemPrompt: sysPrompt,
		Models:       tomlConfig.Models,
	}, nil
}

// readTomlConfig decodes the TOML configuration file at $XDG_CONFIG_HOME/akernel/config.toml.
func readTomlConfig() (TomlConfig, error) {
	tomlConfigPath, err := xdg.ConfigFile("akernel/config.toml")

	var config TomlConfig
	_, err = toml.DecodeFile(tomlConfigPath, &config)
	return config, err
}

// readSystemPrompt reads $XDG_CONFIG_HOME/akernel/system.md and returns its contents.
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
