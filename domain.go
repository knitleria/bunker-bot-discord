package main

import (
	"fmt"
	"strings"
)

const (
	StatGender     = "gender"
	StatAge        = "age"
	StatFertility  = "fertility"
	StatHealth     = "health"
	StatProfession = "profession"
	StatHobby      = "hobby"
	StatBaggage    = "baggage"
	StatFact1      = "fact1"
	StatFact2      = "fact2"
)

var AllStatKeys = []string{StatGender, StatAge, StatFertility, StatHealth, StatProfession, StatHobby, StatBaggage, StatFact1, StatFact2}
var HumanKeys = map[string]string{
	StatGender:     "Пол",
	StatAge:        "Возраст",
	StatFertility:  "Фертильность",
	StatHealth:     "Здоровье",
	StatProfession: "Профессия",
	StatHobby:      "Хобби",
	StatBaggage:    "Багаж",
	StatFact1:      "Факт 1",
	StatFact2:      "Факт 2",
}

var RerollGroups = map[string][]string{
	"biology":    {StatGender, StatAge, StatFertility},
	"health":     {StatHealth},
	"profession": {StatProfession},
	"hobby":      {StatHobby},
	"baggage":    {StatBaggage},
	"facts":      {StatFact1, StatFact2},
}

var PoolForStat = map[string]string{
	StatGender:     "gender",
	StatAge:        "age",
	StatFertility:  "fertility",
	StatHealth:     "health",
	StatProfession: "profession",
	StatHobby:      "hobby",
	StatBaggage:    "baggage",
	StatFact1:      "facts",
	StatFact2:      "facts",
}

type Player struct {
	Nickname  string            `json:"nickname"`
	DiscordID string            `json:"discord_id"`
	Stats     map[string]string `json:"stats"`
}

func (p *Player) Clone() *Player {
	t := make(map[string]string, len(p.Stats))
	for k, v := range p.Stats {
		t[k] = v
	}
	return &Player{Nickname: p.Nickname, DiscordID: p.DiscordID, Stats: t}
}

func formatStats(p *Player) string {
	s := p.Stats
	var b strings.Builder
	b.WriteString("Твоя карточка\n\n")
	fmt.Fprintf(&b, "Биология: %s, %s, %s \n", s[StatGender], s[StatAge], s[StatFertility])
	fmt.Fprintf(&b, "Здоровье: %s \n", s[StatHealth])
	fmt.Fprintf(&b, "Профессия: %s \n", s[StatProfession])
	fmt.Fprintf(&b, "Хобби: %s \n", s[StatHobby])
	fmt.Fprintf(&b, "Багаж: %s \n", s[StatBaggage])
	fmt.Fprintf(&b, "Факт 1: %s \n", s[StatFact1])
	fmt.Fprintf(&b, "Факт 2: %s \n", s[StatFact2])

	return b.String()
}

func formatBoard(players []*Player) string {
	if len(players) == 0 {
		return "Нет игроков. Вызови команду /upload"
	}
	var b strings.Builder
	b.WriteString("Карточки игроков\n\n")
	for _, p := range players {
		fmt.Fprintf(&b, "%s: %s\n", p.Nickname, formatStats(p))
	}
	return b.String()
}
