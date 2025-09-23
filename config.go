package main

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	DataFolder        string       `json:"data_folder"`
	LogFile           string       `json:"log_file"`
	BaseURL           string       `json:"base_url"`
	Style             string       `json:"style"`
	Template          string       `json:"template"`
	Port              string       `json:"port"`
	AdminUser         string       `json:"admin_user"`
	Lang              string       `json:"lang"`
	UploadMax         int64        `json:"upload_max"`
	MaxFailed         int          `json:"max_failed"`
	BlockDurationStr  string       `json:"block_duration"`
	CheckDurationStr  string       `json:"check_duration"`
}

func (c *Config) BlockDuration() time.Duration {
	dur, err := time.ParseDuration(c.BlockDurationStr)
	if err != nil {
		panic(err)
	}
	return dur
}

func (c *Config) CheckDuration() time.Duration {
	dur, err := time.ParseDuration(c.CheckDurationStr)
	if err != nil {
		panic(err)
	}
	return dur
}

func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}