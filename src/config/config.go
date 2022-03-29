/*
 * trellofs - A Trello POSIX filesystem
 * Copyright (C) 2022  Joao Eduardo Luis <joao@wipwd.dev>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 */
package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
)

type Config struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Token string `json:"token"`
}

func ReadConfig(cfg string) (*Config, error) {

	confFile := os.Getenv("TCLI_CONFIG")
	if confFile == "" {
		confFile = cfg
	}

	if _, err := os.Stat(confFile); errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	contents, err := ioutil.ReadFile(confFile)
	if err != nil {
		return nil, err
	}

	config := new(Config)
	json.Unmarshal(contents, config)
	return config, nil
}
