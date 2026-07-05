package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ConfigFile struct {
	Players []*Player           `json:"players"`
	Pools   map[string][]string `json:"pools"`
}

var requiredPools = []string{"gender", "age", "fertility", "health", "profession", "hobby", "baggage", "fact1", "fact2"}

func downloadAttachment(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download attachment: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func parseConfigFile(data []byte) (*ConfigFile, error) {
	var config ConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	if len(config.Players) == 0 {
		return nil, fmt.Errorf("no players in config file")
	}
	if len(config.Pools) == 0 {
		return nil, fmt.Errorf("no pools in config file")
	}

	var missingPools []string
	for _, k := range requiredPools {
		if len(config.Pools[k]) == 0 {
			missingPools = append(missingPools, k)
		}
	}
	if len(missingPools) > 0 {
		return nil, fmt.Errorf("missing pools: %v", strings.Join(missingPools, ", "))
	}
	for _, p := range config.Players {
		if p.Stats == nil {
			p.Stats = map[string]string{}
		}
	}
	return &config, nil
}
