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
)

func TestPrintCard(t *testing.T) {
	tables := []struct {
		file   string
		output string
	}{
		//{"test_data/ponder.json", "Ponder {U} |Sorcery| Look at the top three cards of your library, then put them back in any order. You may shuffle your library. / Draw a card. · C18-C,LRW-C,M10-C,M12-C,MPR-P,P08-P · Vin,VinRes,Leg,ModBan"},
		{"test_data/ponder.json", "\x02Ponder\x0F {U} | Sorcery | Look at the top three cards of your library, then put them back in any order. You may shuffle your library. \\ Draw a card. · C18-C · VinRes,Leg,ModBan"},
		//{"test_data/shahrazad.json", "Shahrazad {WW} |Sorcery| Players play a Magic subgame, using their libraries as their decks. Each player who doesn't win the subgame loses half their life, rounded up. · AN-R,AN-U2,Reserved · VinBan,LegBan"},
		{"test_data/shahrazad.json", "\x02Shahrazad\x0F {WW} | Sorcery | Players play a Magic subgame, using their libraries as their decks. Each player who doesn't win the subgame loses half their life, rounded up. · ARN-R · VinBan,LegBan"},
		{"test_data/jacethemindsculptor.json", "\x02Jace, the Mind Sculptor\x0F {2UU} | Legendary Planeswalker — Jace | [3] +2: Look at the top card of target player's library. You may put that card on the bottom of that player's library. \\ 0: Draw three cards, then put two cards from your hand on top of your library in any order. \\ −1: Return target creature to its owner's hand. \\ −12: Exile all cards from target player's library, then that player shuffles their hand into their library. · A25-M · Vin,Leg,Mod"},
		{"test_data/expansion.json", "\x02Expansion\x0F {U/RU/R} | Instant | Copy target instant or sorcery spell with converted mana cost 4 or less. You may choose new targets for the copy. · GRN-R · Vin,Leg,Mod,Std\n\x02Explosion\x0F {XUURR} | Instant | Explosion deals X damage to any target. Target player draws X cards. · GRN-R · Vin,Leg,Mod,Std"},
	}
	for _, table := range tables {
		fi, err := os.Open(table.file)
		if err != nil {
			t.Errorf("Unable to open %v", table.file)
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
