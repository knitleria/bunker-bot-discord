package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Game struct {
	Players []*Player         `json:"players"`
	Pools   map[string]string `json:"pools"`
	Started bool              `json:"started"`
}

type Store struct {
	mu    sync.Mutex
	dir   string
	games map[string]*Game
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	s := &Store{
		dir:   dir,
		games: make(map[string]*Game),
	}
	if err := s.loadAll(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) loadAll() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			return err
		}
		g := &Game{}
		if err := json.Unmarshal(data, g); err != nil {
			return err
		}
		s.games[strings.TrimSuffix(entry.Name(), ".json")] = g
	}
	return nil
}

func (s *Store) gameLocked(guildID string) *Game {
	g := s.games[guildID]
	if g == nil {
		g = &Game{}
		s.games[guildID] = g
	}
	return g
}

func (s *Store) saveLocked(guildID string) error {
	data, err := json.MarshalIndent(s.games[guildID], "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, guildID+".json"), data, 0644)
}

func (s *Store) Update(guildID string, f func(*Game) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.gameLocked(guildID)
	if err := f(g); err != nil {
		return err
	}
	return s.saveLocked(guildID)
}

func (s *Store) Read(guildID string, f func(*Game) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.gameLocked(guildID)
	if err := f(g); err != nil {
		return err
	}
	return nil
}
