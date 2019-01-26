package main

import (
	"encoding/json"
	"os"
	"testing"
)

var (
	TestCardWithNoRulings        = Card{Name: "TestCardWithNoRulings"}
	TestCardWithEmptyRulings     = Card{Name: "TestCardWithNoRulings", Rulings: []CardRuling{}}
	TestCardWithOneNonWOTCRuling = Card{Name: "TestCardWithOneNonWOTCRuling", Rulings: []CardRuling{{Source: "Fry", Comment: "No Print!"}}}
	TestCardWithOneWOTCRuling    = Card{Name: "TestCardWithOneWOTCRuling", Rulings: []CardRuling{{Source: "wotc", Comment: "Print Me", PublishedAt: "1900-01-01"}}}
	TestCardWithTwoWOTCRulings   = Card{Name: "TestCardWithTwoWOTCRulings", Rulings: []CardRuling{{Source: "wotc", Comment: "Print Me", PublishedAt: "1900-02-02"}, {Source: "wotc", Comment: "Print Me Too!", PublishedAt: "1900-03-03"}}}
	FakeCards                    = []Card{TestCardWithNoRulings, TestCardWithEmptyRulings, TestCardWithOneNonWOTCRuling, TestCardWithOneWOTCRuling, TestCardWithTwoWOTCRulings}
	RealCards                    = map[string]string{
		"Ponder":                  "test_data/ponder.json",
		"Shahrazad":               "test_data/shahrazad.json",
		"Jace, the Mind Sculptor": "test_data/jacethemindsculptor.json",
		"Expansion":               "test_data/expansion.json",
		"Bushi Tenderfoot":        "test_data/bushitenderfoot.json",
		"Claim":                   "test_data/claimtofame.json",
		"Poison-Tip Archer":       "test_data/poisontiparcher.json",
		"Faithless Looting":       "test_data/faithlesslooting.json",
	}
)

func TestPrintCard(t *testing.T) {
	tables := []struct {
		cardname string
		output   string
	}{
		{"Ponder", "\x02Ponder\x0F {U} | Sorcery | Look at the top three cards of your library, then put them back in any order. You may shuffle your library. \\ Draw a card. · C18-C · VinRes,Leg,ModBan"},
		{"Shahrazad", "\x02Shahrazad\x0F {W}{W} | Sorcery | Players play a Magic subgame, using their libraries as their decks. Each player who doesn't win the subgame loses half their life, rounded up. · ARN-R · VinBan,LegBan"},
		{"Jace, the Mind Sculptor", "\x02Jace, the Mind Sculptor\x0F {2}{U}{U} | Legendary Planeswalker — Jace | [3] +2: Look at the top card of target player's library. You may put that card on the bottom of that player's library. \\ 0: Draw three cards, then put two cards from your hand on top of your library in any order. \\ −1: Return target creature to its owner's hand. \\ −12: Exile all cards from target player's library, then that player shuffles their hand into their library. · A25-M · Vin,Leg,Mod"},
		{"Expansion", "\x02Expansion\x0F {U/R}{U/R} | Instant | Copy target instant or sorcery spell with converted mana cost 4 or less. You may choose new targets for the copy. · GRN-R · Vin,Leg,Mod,Std\n\x02Explosion\x0F {X}{U}{U}{R}{R} | Instant | Explosion deals X damage to any target. Target player draws X cards. · GRN-R · Vin,Leg,Mod,Std"},
		{"Bushi Tenderfoot", "\x02Bushi Tenderfoot\x0F {W} | Creature — Human Soldier | 1/1 When a creature dealt damage by Bushi Tenderfoot this turn dies, flip Bushi Tenderfoot. · CHK-U · Vin,Leg,Mod\n\x02Kenzo the Hardhearted\x0F | Legendary Creature — Human Samurai | 3/4 Double strike; bushido 2 (Whenever this creature blocks or becomes blocked, it gets +2/+2 until end of turn.) · CHK-U · Vin,Leg,Mod"},
	}
	for _, table := range tables {
		fi, err := os.Open(RealCards[table.cardname])
		if err != nil {
			t.Errorf("Unable to open %v", RealCards[table.cardname])
		}
		var c Card
		if err := json.NewDecoder(fi).Decode(&c); err != nil {
			t.Errorf("Something went wrong parsing the card: %s", err)
		}
		fc := c.formatCard()
		if fc != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", fc, table.output)
		}
	}
}

func TestGetReminders(t *testing.T) {
	tables := []struct {
		cardname string
		output   string
	}{
		{"Ponder", "Reminder text not found"},
		{"Poison-Tip Archer", "This creature can block creatures with flying.\nAny amount of damage this deals to a creature is enough to destroy it."},
		{"Claim", "Cast this spell only from your graveyard. Then exile it."},
		{"Faithless Looting", "You may cast this card from your graveyard for its flashback cost. Then exile it."},
	}
	for _, table := range tables {
		fi, err := os.Open(RealCards[table.cardname])
		if err != nil {
			t.Errorf("Unable to open %v", RealCards[table.cardname])
		}
		var c Card
		if err := json.NewDecoder(fi).Decode(&c); err != nil {
			t.Errorf("Something went wrong parsing the card: %s", err)
		}
		fc := c.getReminderTexts()
		if fc != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", fc, table.output)
		}
	}
}

func TestGetFlavour(t *testing.T) {
	tables := []struct {
		cardname string
		output   string
	}{
		{"Ponder", "Tomorrow belongs to those who prepare for it today."},
		{"Poison-Tip Archer", "Flavour text not found"},
	}
	for _, table := range tables {
		fi, err := os.Open(RealCards[table.cardname])
		if err != nil {
			t.Errorf("Unable to open %v", RealCards[table.cardname])
		}
		var c Card
		if err := json.NewDecoder(fi).Decode(&c); err != nil {
			t.Errorf("Something went wrong parsing the card: %s", err)
		}
		fc := c.getFlavourText()
		if fc != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", fc, table.output)
		}
	}
}

func TestGetRulings(t *testing.T) {
	tables := []struct {
		input        Card
		rulingNumber int
		output       string
	}{
		{TestCardWithNoRulings, 0, "Problem fetching the rulings"},
		{TestCardWithNoRulings, 1, "Problem fetching the rulings"},
		{TestCardWithEmptyRulings, 0, "Ruling not found"},
		{TestCardWithEmptyRulings, 1, "Ruling not found"},
		{TestCardWithOneNonWOTCRuling, 0, "Ruling not found"},
		{TestCardWithOneWOTCRuling, 0, "1900-01-01: Print Me"},
		{TestCardWithOneWOTCRuling, 1, "1900-01-01: Print Me"},
		{TestCardWithTwoWOTCRulings, 0, "1900-02-02: Print Me\n1900-03-03: Print Me Too!"},
		{TestCardWithTwoWOTCRulings, 1, "1900-02-02: Print Me"},
		{TestCardWithTwoWOTCRulings, 2, "1900-03-03: Print Me Too!"},
	}
	for _, table := range tables {
		got := (table.input).getRulings(table.rulingNumber)
		if got != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", got, table.output)
		}
	}
}
