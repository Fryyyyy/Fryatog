package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPrintCard(t *testing.T) {
	tables := []struct {
		file   string
		output string
	}{
		{"test_data/ponder.json", "Ponder {U} |Sorcery| Look at the top three cards of your library, then put them back in any order. You may shuffle your library. / Draw a card. · C18-C,LRW-C,M10-C,M12-C,MPR-P,P08-P · Vin,VinRes,Leg,ModBan"},
		{"test_data/shahrazad.json", "Shahrazad {WW} |Sorcery| Players play a Magic subgame, using their libraries as their decks. Each player who doesn't win the subgame loses half their life, rounded up. · AN-R,AN-U2,Reserved · VinBan,LegBan"},
		{"test_data/jacethemindsculptor.json", "Jace, the Mind Sculptor {2UU} |Legendary Planeswalker -- Jace| 3 loyalty. +2: Look at the top card of target player's library. You may put that card on the bottom of that player's library. / 0: Draw three cards, then put two cards from your hand on top of your library in any order. / -1: Return target creature to its owner's hand. / -12: Exile all cards from target player's library, then that ...\n... player shuffles their hand into their library. · A25-M,EMA-M,V13-M,VMA-M,WWK-M · Vin,Leg,Mod"},
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
		fc := formatCard(&c)
		if fc != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", fc, table.output)
		}
	}
}
