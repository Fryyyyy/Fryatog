package main

import (
	"encoding/json"
	"fmt"
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
		"Fleetwheel Cruiser":      "test_data/fleetwheelcruiser.json",
		"Disenchant":              "test_data/disenchant.json",
	}
)

func (card *Card) fakeGetMetadata() error {
	var cm CardMetadata
	var list CardList
	fi, err := os.Open("test_data/" + normaliseCardName(card.Name) + "-printings.json")
	if err != nil {
		return fmt.Errorf("Unable to open printings JSON: %v", err)
	}
	if err := json.NewDecoder(fi).Decode(&list); err != nil {
		return fmt.Errorf("Something went wrong parsing the card list")
	}
	if len(list.Warnings) > 0 {
		return fmt.Errorf("Scryfall said there were errors: %v", list.Warnings)
	}
	// These are in printing order, since the prints_search_uri includes "order=released"
	for _, c := range list.Data {
		if c.ID == card.ID {
			continue
		}
		if c.FlavourText != "" {
			cm.PreviousFlavourTexts = append(cm.PreviousFlavourTexts, c.FlavourText)
		}
		cm.PreviousPrintings = append(cm.PreviousPrintings, c.formatExpansions())
		// Only need previous reminder text if current one doesn't have
		if c.getReminderTexts() == "Reminder text not found" && c.getReminderTexts() != "Reminder text not found" {
			cm.PreviousReminderTexts = append(cm.PreviousReminderTexts, c.getReminderTexts())
		}
	}
	card.Metadata = cm
	return nil
}

func TestPrintCard(t *testing.T) {
	tables := []struct {
		cardname string
		output   string
	}{
		{"Ponder", "\x02Ponder\x0F {U} · Sorcery · Look at the top three cards of your library, then put them back in any order. You may shuffle your library. \\ Draw a card. · C18-C · VinRes,Leg,ModBan"},
		{"Shahrazad", "\x02Shahrazad\x0F {W}{W} · Sorcery · Players play a Magic subgame, using their libraries as their decks. Each player who doesn't win the subgame loses half their life, rounded up. · ARN-R · VinBan,LegBan"},
		{"Jace, the Mind Sculptor", "\x02Jace, the Mind Sculptor\x0F {2}{U}{U} · Legendary Planeswalker — Jace · [3] +2: Look at the top card of target player's library. You may put that card on the bottom of that player's library. \\ 0: Draw three cards, then put two cards from your hand on top of your library in any order. \\ −1: Return target creature to its owner's hand. \\ −12: Exile all cards from target player's library, then that player shuffles their hand into their library. · A25-M · Vin,Leg,Mod"},
		{"Expansion", "\x02Expansion\x0F {U/R}{U/R} · Instant · Copy target instant or sorcery spell with converted mana cost 4 or less. You may choose new targets for the copy. · GRN-R · Vin,Leg,Mod,Std\n\x02Explosion\x0F {X}{U}{U}{R}{R} · Instant · Explosion deals X damage to any target. Target player draws X cards. · GRN-R · Vin,Leg,Mod,Std"},
		{"Bushi Tenderfoot", "\x02Bushi Tenderfoot\x0F {W} · Creature — Human Soldier · 1/1 · When a creature dealt damage by Bushi Tenderfoot this turn dies, flip Bushi Tenderfoot. · CHK-U · Vin,Leg,Mod\n\x02Kenzo the Hardhearted\x0F · Legendary Creature — Human Samurai · 3/4 · Double strike; bushido 2 (Whenever this creature blocks or becomes blocked, it gets +2/+2 until end of turn.) · CHK-U · Vin,Leg,Mod"},
		{"Fleetwheel Cruiser", "\x02Fleetwheel Cruiser\x0F {4} · Artifact — Vehicle · 5/3 · Trample, haste \\ When Fleetwheel Cruiser enters the battlefield, it becomes an artifact creature until end of turn. \\ Crew 2 (Tap any number of creatures you control with total power 2 or more: This Vehicle becomes an artifact creature until end of turn.) · KLD-R · Vin,Leg,Mod"},
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

func TestGetPrintings(t *testing.T) {
	tables := []struct {
		cardname string
		output   string
	}{
		{"Faithless Looting", "\x02Faithless Looting\x0F {R} · Sorcery · Draw two cards, then discard two cards. \\ Flashback {2}{R} (You may cast this card from your graveyard for its flashback cost. Then exile it.) · CM2-C,EMA-C,PZ1-U,C15-C,C14-C,DDK-C,DKA-C,PIDW-R,UMA-C · Vin,Leg,Mod"},
		{"Disenchant", "\x02Disenchant\x0F {1}{W} · Instant · Destroy target artifact or enchantment. · IMA-C,PRM-C,CN2-C,TPR-C,PRM-C,[...],A25-C · Vin,Leg,Mod"},
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
		if err = c.fakeGetMetadata(); err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		fc := c.formatCard()
		if fc != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", fc, table.output)
		}
	}
}
