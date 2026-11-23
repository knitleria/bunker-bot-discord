package main

import (
	"fmt"
	"math/rand"
	"strings"
)

func uniqueDraw(pool []string, n int, rng *rand.Rand) ([]string, error) {
	if n < 0 {
		return nil, fmt.Errorf("draw count must not be negative")
	}
	if rng == nil {
		return nil, fmt.Errorf("random source is required")
	}
	values, err := normalizePool(pool)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, n)
	for len(out) < n {
		order := rng.Perm(len(values))
		// Avoid equal neighbours at a cycle boundary. In particular, a fact
		// pair straddling two cycles must still contain different values.
		if len(values) > 1 && len(out) > 0 && values[order[0]] == out[len(out)-1] {
			j := 1 + rng.Intn(len(order)-1)
			order[0], order[j] = order[j], order[0]
		}
		for _, index := range order {
			if len(out) == n {
				break
			}
			out = append(out, values[index])
		}
	}
	return out, nil
}

func dealAll(players []*Player, pools map[string][]string, rng *rand.Rand) error {
	return rerollGroup(players, "all", pools, rng)
}

func rerollGroup(players []*Player, group string, pools map[string][]string, rng *rand.Rand) error {
	keys, ok := RerollGroups[group]
	if group == "all" {
		keys = AllStatKeys
	} else if !ok {
		return fmt.Errorf("unknown reroll group: %q", group)
	}
	if err := validatePlayers(players); err != nil {
		return err
	}
	if rng == nil {
		return fmt.Errorf("random source is required")
	}

	poolKeys := make([]string, 0, len(keys))
	seen := make(map[string]bool)
	for _, key := range keys {
		pool := PoolForStat[key]
		if !seen[pool] {
			poolKeys = append(poolKeys, pool)
			seen[pool] = true
		}
	}
	normalized, err := normalizePools(pools, poolKeys)
	if err != nil {
		return err
	}

	// Prepare every draw before touching players, so invalid input cannot
	// leave partially updated cards, even with the current Store.Update.
	draws := make(map[string][]string, len(normalized))
	for _, pool := range poolKeys {
		n := len(players)
		if pool == "facts" {
			n *= 2
		}
		draws[pool], err = uniqueDraw(normalized[pool], n, rng)
		if err != nil {
			return err
		}
	}
	for i, p := range players {
		if p.Stats == nil {
			p.Stats = make(map[string]string)
		}
		for _, key := range keys {
			index := i
			if key == StatFact1 {
				index = 2 * i
			} else if key == StatFact2 {
				index = 2*i + 1
			}
			p.Stats[key] = draws[PoolForStat[key]][index]
		}
	}
	return nil
}

func findPlayer(g *Game, nickname string) *Player {
	for _, p := range g.Players {
		if strings.EqualFold(p.Nickname, nickname) {
			return p
		}
	}
	return nil
}
