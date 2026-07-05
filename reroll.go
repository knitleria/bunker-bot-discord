package main

import (
	"math/rand"
	"strings"
)

func uniqueDraw(pool []string, n int) []string {
	if len(pool) == 0 {
		out := make([]string, n)
		for i := range out {
			out[i] = "-"
		}
		return out
	}

	out := make([]string, n)
	for len(out) < n {
		for _, item := range rand.Perm(len(pool)) {
			if len(out) >= n {
				break
			}
			out = append(out, pool[item])
		}
	}
	rand.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out
}

func assignColumn(players []*Player, statKey string, pool []string) {
	value := uniqueDraw(pool, len(players))
	for i, p := range players {
		if p.Stats == nil {
			p.Stats = map[string]string{}
		}
		p.Stats[statKey] = value[i]
	}
}

func assignFacts(players []*Player, pool []string) {
	value := uniqueDraw(pool, len(players))
	for i, p := range players {
		if p.Stats == nil {
			p.Stats = map[string]string{}
		}
		p.Stats[StatFact1] = value[i*2]
		p.Stats[StatFact2] = value[i*2+1]
	}
}

func dealAll(players []*Player, pools map[string][]string) {
	assignColumn(players, StatGender, pools["gender"])
	assignColumn(players, StatAge, pools["age"])
	assignColumn(players, StatFertility, pools["fertility"])
	assignColumn(players, StatHealth, pools["health"])
	assignColumn(players, StatProfession, pools["profession"])
	assignColumn(players, StatHobby, pools["hobby"])
	assignColumn(players, StatBaggage, pools["baggage"])
	assignFacts(players, pools["facts"])

}

func rerollGroup(players []*Player, group string, pools map[string][]string) {
	switch group {
	case "all":
		dealAll(players, pools)
	case "biology":
		assignColumn(players, StatGender, pools["gender"])
		assignColumn(players, StatAge, pools["age"])
		assignColumn(players, StatFertility, pools["fertility"])
	case "facts":
		assignFacts(players, pools["facts"])
	default:
		for _, key := range RerollGroups[group] {
			assignColumn(players, key, pools[PoolForStat[key]])
		}
	}
}

func findPlayer(g *Game, nickname string) *Player {
	for _, p := range g.Players {
		if strings.EqualFold(p.Nickname, nickname) {
			return p
		}
	}
	return nil
}
