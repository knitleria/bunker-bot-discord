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

var requiredPools = []string{"gender", "age", "fertility", "health", "profession", "hobby", "baggage", "facts"}

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
	if err := validatePlayers(config.Players); err != nil {
		return nil, err
	}
	pools, err := normalizePools(config.Pools, requiredPools)
	if err != nil {
		return nil, err
	}
	config.Pools = pools
	for _, p := range config.Players {
		p.Nickname = strings.TrimSpace(p.Nickname)
		p.DiscordID = strings.TrimSpace(p.DiscordID)
		if p.Stats == nil {
			p.Stats = map[string]string{}
		}
	}
	return &config, nil
}

func validatePlayers(players []*Player) error {
	if len(players) == 0 {
		return fmt.Errorf("no players")
	}
	ids := make(map[string]bool)
	var nicknames []string
	for i, p := range players {
		if p == nil {
			return fmt.Errorf("player %d is null", i+1)
		}
		nickname := strings.TrimSpace(p.Nickname)
		if nickname == "" {
			return fmt.Errorf("player %d has an empty nickname", i+1)
		}
		for _, previous := range nicknames {
			if strings.EqualFold(previous, nickname) {
				return fmt.Errorf("duplicate nickname: %q", nickname)
			}
		}
		nicknames = append(nicknames, nickname)
		id := strings.TrimSpace(p.DiscordID)
		if id != "" {
			if ids[id] {
				return fmt.Errorf("duplicate Discord ID: %q", id)
			}
			ids[id] = true
		}
	}
	return nil
}

// normalizePools copies the requested pools, trims values and removes exact
// duplicates. Repeated source rows must not change the odds of a draw.
func normalizePools(pools map[string][]string, keys []string) (map[string][]string, error) {
	out := make(map[string][]string, len(keys))
	for _, key := range keys {
		values, err := normalizePool(pools[key])
		if err != nil {
			return nil, fmt.Errorf("pool %q: %w", key, err)
		}
		if key == "facts" && len(values) < 2 {
			return nil, fmt.Errorf("pool %q needs at least two distinct values", key)
		}
		out[key] = values
	}
	return out, nil
}

func normalizePool(pool []string) ([]string, error) {
	if len(pool) == 0 {
		return nil, fmt.Errorf("pool is missing or empty")
	}
	out := make([]string, 0, len(pool))
	seen := make(map[string]bool)
	for _, value := range pool {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("pool contains an empty value")
		}
		if !seen[value] {
			out = append(out, value)
			seen[value] = true
		}
	}
	return out, nil
}
