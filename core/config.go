package core

import (
	"encoding/json"
	"os"
	"time"
)

// a struct to hold the data of `etc/config.json`
type Config struct {
	DataFolder        string       `json:"data_folder"`
	LogFile           string       `json:"log_file"`
	BaseURL           string       `json:"base_url"`
	LinkPrefix        string       `json:"link_prefix"`
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

// It is important to use this function instead of `BlockDurationStr`. Otherwise "30m" maybe 30 us and not 30 minutes.
func (c *Config) BlockDuration() time.Duration {
	dur, err := time.ParseDuration(c.BlockDurationStr)
	if err != nil {
		panic(err)
	}
	return dur
}

// It is important to use this function instead of `CheckDurationStr`.
// Otherwise "5m" maybe 5 us and not 5 minutes and the cleanup thread takes 100% cpu.
func (c *Config) CheckDuration() time.Duration {
	dur, err := time.ParseDuration(c.CheckDurationStr)
	if err != nil {
		panic(err)
	}
	return dur
}

// It loads a given json file and tries to place its data inside the Config struct (and returns it or `nil` with error).
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