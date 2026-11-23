package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func exampleConfig(t *testing.T) *ConfigFile {
	t.Helper()
	data, err := os.ReadFile("testdata/config.example.json")
	if err != nil {
		t.Fatal(err)
	}
	cf, err := parseConfigFile(data)
	if err != nil {
		t.Fatal(err)
	}
	return cf
}

func TestParseConfigAllowsUnlinkedPlayers(t *testing.T) {
	cf := exampleConfig(t)
	cf.Players = []*Player{
		{Nickname: " Alice ", DiscordID: "", Stats: map[string]string{StatHealth: "Custom health"}},
		{Nickname: "Bob", DiscordID: " \t"},
	}
	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseConfigFile(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range parsed.Players {
		if p.DiscordID != "" {
			t.Fatalf("unlinked player has ID %q", p.DiscordID)
		}
	}
	if !reflect.DeepEqual(parsed.Players[0].Stats, cf.Players[0].Stats) {
		t.Fatal("import changed existing stats")
	}
}

func TestParseConfigRequiresEveryPool(t *testing.T) {
	for _, key := range []string{"gender", "age", "fertility", "health", "profession", "hobby", "baggage", "facts"} {
		t.Run(key, func(t *testing.T) {
			cf := exampleConfig(t)
			delete(cf.Pools, key)
			data, err := json.Marshal(cf)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parseConfigFile(data); err == nil {
				t.Fatalf("accepted config without %s", key)
			}
		})
	}
}

func TestParseConfigNormalizesInput(t *testing.T) {
	cf := exampleConfig(t)
	cf.Players[0].Nickname = "  Иван  "
	cf.Players[0].DiscordID = " 123 "
	cf.Pools["health"] = []string{" Здоров ", "Здоров", "Астма"}
	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseConfigFile(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Players[0].Nickname != "Иван" || parsed.Players[0].DiscordID != "123" {
		t.Fatalf("player was not trimmed: %+v", parsed.Players[0])
	}
	if len(parsed.Pools) != 8 || len(parsed.Pools["health"]) != 2 || parsed.Pools["health"][0] != "Здоров" {
		t.Fatalf("unexpected normalized pools: %v", parsed.Pools)
	}
	for _, p := range parsed.Players {
		if p.Stats == nil {
			t.Fatal("stats map was not initialized")
		}
	}
}

func TestParseConfigRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name   string
		change func(*ConfigFile)
	}{
		{"no players", func(c *ConfigFile) { c.Players = nil }},
		{"null player", func(c *ConfigFile) { c.Players[0] = nil }},
		{"empty nickname", func(c *ConfigFile) { c.Players[0].Nickname = " \t" }},
		{"duplicate nickname", func(c *ConfigFile) { c.Players[1].Nickname = " иВАН " }},
		{"duplicate ID", func(c *ConfigFile) { c.Players[0].DiscordID = "123"; c.Players[1].DiscordID = " 123 " }},
		{"no pools", func(c *ConfigFile) { c.Pools = nil }},
		{"missing pool", func(c *ConfigFile) { delete(c.Pools, "age") }},
		{"empty pool", func(c *ConfigFile) { c.Pools["hobby"] = []string{} }},
		{"blank value", func(c *ConfigFile) { c.Pools["health"] = []string{"Здоров", " \n"} }},
		{"one distinct fact", func(c *ConfigFile) { c.Pools["facts"] = []string{"Факт", " Факт "} }},
		{"old fact pools", func(c *ConfigFile) {
			delete(c.Pools, "facts")
			c.Pools["fact1"] = []string{"A"}
			c.Pools["fact2"] = []string{"B"}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cf := exampleConfig(t)
			tc.change(cf)
			data, err := json.Marshal(cf)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parseConfigFile(data); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	for _, data := range []string{"null", "{", "{}"} {
		if _, err := parseConfigFile([]byte(data)); err == nil {
			t.Fatalf("accepted invalid config %q", data)
		}
	}
}
