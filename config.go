package main

import (
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

var config Config

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
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Sensitive   bool   `yaml:"sensitive"`
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
	ProcessWith string `yaml:"process_with,omitempty"`
}

type Config struct {
	LogLevel string     `yaml:"log_level"`
	LogJSON  bool       `yaml:"log_json"`
	Outbound []Outbound `yaml:"outbound"`
	Inbound  []Inbound  `yaml:"inbound"`
	Remotes  []Remote   `yaml:"remotes"`
}

func readConfig(filename string) error {
	// Read YAML config file
	fullpath, _ := filepath.Abs(filename)
	yamlFile, err := ioutil.ReadFile(fullpath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return err
	}
	return nil
}
