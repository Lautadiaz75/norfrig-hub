package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Root struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type Config struct {
	Roots []Root `yaml:"roots"`
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	DB struct {
		Path string `yaml:"path"`
	} `yaml:"db"`
	Thumbnails struct {
		Dir  string `yaml:"dir"`
		Size int    `yaml:"size"`
	} `yaml:"thumbnails"`
	Indexer struct {
		Workers int `yaml:"workers"`
	} `yaml:"indexer"`
	SKU struct {
		Blacklist []string `yaml:"blacklist"`
	} `yaml:"sku"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.DB.Path == "" {
		cfg.DB.Path = "data/hub.db"
	}
	if cfg.Indexer.Workers == 0 {
		cfg.Indexer.Workers = 4
	}
	if cfg.Thumbnails.Size == 0 {
		cfg.Thumbnails.Size = 256
	}
	if cfg.Thumbnails.Dir == "" {
		cfg.Thumbnails.Dir = "data/thumbs"
	}
	return &cfg, nil
}
