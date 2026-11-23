package main

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func testRNG(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

func TestUniqueDrawSingleValue(t *testing.T) {
	for _, n := range []int{0, 1, 10} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			values, err := uniqueDraw([]string{"only", "only"}, n, testRNG(1))
			if err != nil {
				t.Fatal(err)
			}
			if len(values) != n {
				t.Fatalf("got %d values, want %d", len(values), n)
			}
			for _, value := range values {
				if value != "only" {
					t.Fatalf("unexpected value %q", value)
				}
			}
		})
	}
}

func TestRerollMayKeepPreviousValue(t *testing.T) {
	players := []*Player{{Nickname: "Alice", Stats: map[string]string{StatHealth: "Healthy", StatHobby: "Chess"}}}
	if err := rerollGroup(players, "health", map[string][]string{"health": {"Healthy"}}, testRNG(1)); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{StatHealth: "Healthy", StatHobby: "Chess"}
	if !reflect.DeepEqual(players[0].Stats, want) {
		t.Fatalf("unexpected stats: %v", players[0].Stats)
	}
}

func TestDealAllNormalizesPoolsWithoutChangingInputs(t *testing.T) {
	pools := map[string][]string{
		"gender": {" M ", "M"}, "age": {" 30 "}, "fertility": {"Yes"},
		"health": {"Healthy"}, "profession": {"Doctor"}, "hobby": {"Chess"},
		"baggage": {"Radio"}, "facts": {" First ", "First", "Second", " Second "},
	}
	before := make(map[string][]string, len(pools))
	for key, values := range pools {
		before[key] = append([]string(nil), values...)
	}
	players := []*Player{{Nickname: "Alice"}, {Nickname: "Bob"}, {Nickname: "Carol"}}
	if err := dealAll(players, pools, testRNG(1)); err != nil {
		t.Fatal(err)
	}
	for _, p := range players {
		if p.Stats[StatGender] != "M" || p.Stats[StatAge] != "30" {
			t.Fatalf("unnormalized stats: %v", p.Stats)
		}
		facts := map[string]bool{p.Stats[StatFact1]: true, p.Stats[StatFact2]: true}
		if len(facts) != 2 || !facts["First"] || !facts["Second"] {
			t.Fatalf("expected two distinct facts: %v", p.Stats)
		}
	}
	if !reflect.DeepEqual(pools, before) {
		t.Fatal("generation mutated input pools")
	}
}

func TestUniqueDraw(t *testing.T) {
	for _, n := range []int{0, 1, 3, 8, 40} {
		values, err := uniqueDraw([]string{" a ", "b", "c", "a"}, n, testRNG(1))
		if err != nil {
			t.Fatal(err)
		}
		if len(values) != n {
			t.Fatalf("draw length = %d, want %d", len(values), n)
		}
		counts := map[string]int{"a": 0, "b": 0, "c": 0}
		for _, value := range values {
			if _, ok := counts[value]; !ok {
				t.Fatalf("unexpected value %q", value)
			}
			counts[value]++
		}
		for _, count := range counts {
			if count < n/3 || count > (n+2)/3 {
				t.Fatalf("unbalanced draw: %v", counts)
			}
		}
	}
	for _, tc := range []struct {
		pool []string
		n    int
		rng  *rand.Rand
	}{
		{nil, 1, testRNG(1)},
		{[]string{" "}, 1, testRNG(1)},
		{[]string{"a"}, -1, testRNG(1)},
		{[]string{"a"}, 1, nil},
	} {
		if _, err := uniqueDraw(tc.pool, tc.n, tc.rng); err == nil {
			t.Fatal("expected invalid draw to fail")
		}
	}
}

func TestDealAll(t *testing.T) {
	for _, n := range []int{0, 1, 4, 20} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			cf := exampleConfig(t)
			players := make([]*Player, n)
			for i := range players {
				players[i] = &Player{Nickname: fmt.Sprint(i)}
			}
			err := dealAll(players, cf.Pools, testRNG(42))
			if n == 0 {
				if err == nil {
					t.Fatal("expected no-player error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range players {
				for _, key := range AllStatKeys {
					if p.Stats[key] == "" {
						t.Fatalf("missing %s for %s", key, p.Nickname)
					}
				}
				if p.Stats[StatFact1] == p.Stats[StatFact2] {
					t.Fatal("equal facts")
				}
			}
			// Pools large enough for this table must not repeat values.
			for _, key := range AllStatKeys {
				pool := PoolForStat[key]
				needed := n
				if pool == "facts" {
					needed *= 2
				}
				if len(cf.Pools[pool]) < needed {
					continue
				}
				seen := make(map[string]bool)
				keys := []string{key}
				if pool == "facts" {
					keys = []string{StatFact1, StatFact2}
				}
				for _, p := range players {
					for _, k := range keys {
						value := p.Stats[k]
						if seen[value] {
							t.Fatalf("duplicate %s: %q", pool, value)
						}
						seen[value] = true
					}
				}
			}
		})
	}
}

func TestSmallFactPools(t *testing.T) {
	for size := 2; size <= 7; size++ {
		pool := make([]string, size)
		for i := range pool {
			pool[i] = fmt.Sprint(i)
		}
		for seed := int64(0); seed < 100; seed++ {
			players := make([]*Player, 11)
			for i := range players {
				players[i] = &Player{Nickname: fmt.Sprint(i)}
			}
			if err := rerollGroup(players, "facts", map[string][]string{"facts": pool}, testRNG(seed)); err != nil {
				t.Fatal(err)
			}
			counts := make(map[string]int)
			for _, p := range players {
				if p.Stats[StatFact1] == p.Stats[StatFact2] {
					t.Fatalf("equal facts: pool size %d, seed %d", size, seed)
				}
				counts[p.Stats[StatFact1]]++
				counts[p.Stats[StatFact2]]++
			}
			for _, value := range pool {
				if counts[value] < 22/size || counts[value] > (22+size-1)/size {
					t.Fatalf("unbalanced facts: %v", counts)
				}
			}
		}
	}
}

func TestRerollOnlyChangesSelectedGroup(t *testing.T) {
	for group, keys := range RerollGroups {
		t.Run(group, func(t *testing.T) {
			cf := exampleConfig(t)
			for _, p := range cf.Players {
				for _, key := range AllStatKeys {
					p.Stats[key] = "previous " + key
				}
			}
			if err := rerollGroup(cf.Players, group, cf.Pools, testRNG(1)); err != nil {
				t.Fatal(err)
			}
			selected := make(map[string]bool)
			for _, key := range keys {
				selected[key] = true
			}
			for _, p := range cf.Players {
				for _, key := range AllStatKeys {
					changed := p.Stats[key] != "previous "+key
					if changed != selected[key] {
						t.Fatalf("unexpected change to %s for group %s", key, group)
					}
				}
			}
		})
	}
}

func TestGenerationRejectsInputWithoutMutation(t *testing.T) {
	for _, name := range []string{"empty players", "null player", "duplicate nickname", "empty nickname", "duplicate ID", "unknown group", "missing pool", "blank pool", "one fact", "nil random"} {
		t.Run(name, func(t *testing.T) {
			cf := exampleConfig(t)
			group := "all"
			rng := testRNG(1)
			for _, p := range cf.Players {
				p.Stats = map[string]string{StatGender: "Old gender", StatHealth: "Old health", StatFact1: "Old fact"}
			}
			switch name {
			case "empty players":
				cf.Players = nil
			case "null player":
				cf.Players[0] = nil
			case "duplicate nickname":
				cf.Players[1].Nickname = cf.Players[0].Nickname
			case "empty nickname":
				cf.Players[0].Nickname = " "
			case "duplicate ID":
				cf.Players[0].DiscordID = "1"
				cf.Players[1].DiscordID = "1"
			case "unknown group":
				group = "gender"
			case "missing pool":
				delete(cf.Pools, "facts")
			case "blank pool":
				cf.Pools["health"] = []string{" "}
			case "one fact":
				cf.Pools["facts"] = []string{"same", "same"}
			case "nil random":
				rng = nil
			}
			before := make([]*Player, len(cf.Players))
			for i, p := range cf.Players {
				if p != nil {
					before[i] = p.Clone()
				}
			}
			if err := rerollGroup(cf.Players, group, cf.Pools, rng); err == nil {
				t.Fatal("expected validation error")
			}
			if !reflect.DeepEqual(before, append([]*Player{}, cf.Players...)) {
				t.Fatal("invalid input changed players")
			}
		})
	}
}

func TestGenerationReproducible(t *testing.T) {
	a, b := exampleConfig(t), exampleConfig(t)
	if err := dealAll(a.Players, a.Pools, testRNG(7)); err != nil {
		t.Fatal(err)
	}
	if err := dealAll(b.Players, b.Pools, testRNG(7)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed produced different cards")
	}
}
