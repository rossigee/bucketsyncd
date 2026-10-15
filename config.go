// Package main provides the bucket synchronisation service.
package main

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

var config Config
var configMutex sync.RWMutex

type Remote struct {
	Name      string `yaml:"name"`
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"accessKey"`
	SecretKey string `yaml:"secretKey"`
}

type Inbound struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Source      string `yaml:"source"`
	Exchange    string `yaml:"exchange"`
	Queue       string `yaml:"queue"`
	Remote      string `yaml:"remote"`
	Destination string `yaml:"destination"`
}

type Outbound struct {
	Name           string   `yaml:"name"`
	Description    string   `yaml:"description"`
	Sensitive      bool     `yaml:"sensitive"`
	Source         string   `yaml:"source"`
	Destination    string   `yaml:"destination"`
	IgnorePatterns []string `yaml:"ignore_patterns,omitempty"`
	ProcessWith    string   `yaml:"process_with,omitempty"`
}

type Config struct {
	LogLevel            string     `yaml:"log_level"`
	LogJSON             bool       `yaml:"log_json"`
	EnableNotifications bool       `yaml:"enable_notifications"`
	Outbound            []Outbound `yaml:"outbound"`
	Inbound             []Inbound  `yaml:"inbound"`
	Remotes             []Remote   `yaml:"remotes"`
}

func readConfig(filename string) error {
	// Read YAML config file
	fullpath, _ := filepath.Abs(filename)
	// #nosec G304 - This is intentional file reading based on user input
	yamlFile, err := os.ReadFile(fullpath)
	if err != nil {
		return err
	}
	configMutex.Lock()
	err = yaml.Unmarshal(yamlFile, &config)
	configMutex.Unlock()
	if err != nil {
		return err
	}
	return nil
}
