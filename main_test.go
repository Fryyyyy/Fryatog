package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	hbot "github.com/whyrusleeping/hellabot"
)

func fakeGetCard(cardname string) (Card, error) {
	r := rand.Intn(1000)
	fmt.Printf("Trying to get card %v -- Sleeping %v ms\n", cardname, r)
	time.Sleep(time.Duration(r) * time.Millisecond)
	for _, c := range FakeCards {
		if cardname == c.Name {
			return c, nil
		}
	}
	for k, v := range RealCards {
		if cardname == k {
			var c Card
			fi, err := os.Open(v)
			if err != nil {
				return c, fmt.Errorf("Unable to open card JSON: %s", err)
			}
			if err := json.NewDecoder(fi).Decode(&c); err != nil {
				return c, fmt.Errorf("Something went wrong parsing the card: %s", err)
			}
			return c, nil
		}
	}
	return Card{Name: "CARD", Set: "TestSet", Rarity: "TestRare", ID: cardname}, nil
}

func TestNormaliseCardName(t *testing.T) {
	tables := []struct {
		input  string
		output string
	}{
		{"Jace, the Mind Sculptor", "jacethemindsculptor"},
		{"ponder", "ponder"},
	}
	for _, table := range tables {
		got := normaliseCardName(table.input)
		if got != table.output {
			t.Errorf("Incorrect output -- got %s -- want %s", got, table.output)
		}
	}
}

func TestTokens(t *testing.T) {
	// Clear and import rules
	rules = make(map[string][]string)
	err := importRules(false)

	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	var emptyStringSlice []string
	var testCardExpected = "\x02CARD\x0F ·  ·  · TESTSET-T · "
	tables := []struct {
		input  string
		output []string
	}{
		{"Hello! ", emptyStringSlice},
		{"!  ", emptyStringSlice},
		{"Test!", emptyStringSlice},
		{"'Test!'", emptyStringSlice},
		{"What?!? Why does that work", emptyStringSlice},
		{"<Bird12> Just making sure, thank you!!!!", emptyStringSlice},
		{"<Cyclops7> Thank you!! I have one more question kind of in the same realm-- if I want to bring some tokens with me to the same event, am I allowed to keep them in the deckbox with my deck and sideboard, or do I have to keep them someplace else?", emptyStringSlice},
		{"<+mtgrelay> [Fear12] Hi!! Quick question: Does Sundial of the Infinite bypass/combo with Psychic Vortex?", emptyStringSlice},
		{"<+mtgrelay> [Fear12] Hi!! Quick question: Does !Sundial of the Infinite bypass/combo with !Psychic Vortex?", []string{testCardExpected, testCardExpected}},
		{"<MW> !!fract ident &treas nabb", []string{testCardExpected, testCardExpected}},
		{"!cr 100.1a", []string{"\x02100.1a.\x0F A two-player game is a game that begins with only two players."}},
		{"!100.1a !!hi", []string{"\x02100.1a.\x0F A two-player game is a game that begins with only two players.", testCardExpected}},
	}
	for _, table := range tables {
		got := tokeniseAndDispatchInput(&hbot.Message{Content: table.input}, fakeGetCard)
		if !reflect.DeepEqual(got, table.output) {
			t.Errorf("Incorrect output for [%v] -- got %s -- want %s", table.input, got, table.output)
		}
	}
}

func TestRules(t *testing.T) {
	// Clear and import rules
	rules = make(map[string][]string)
	err := importRules(false)

	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	tables := []struct {
		input  string
		output []string
	}{
		{"100.1a", []string{"A two-player game is a game that begins with only two players."}},
		{"Absorb", []string{"\x02Absorb\x0F: A keyword ability that prevents damage. See rule 702.63, \"Absorb.\""}},
		{"ex101.2", []string{`Example: If one effect reads "You may play an additional land this turn" and another reads "You can’t play lands this turn," the effect that precludes you from playing lands wins.`}},
	}
	for _, table := range tables {
		got := rules[table.input]
		if !reflect.DeepEqual(got, table.output) {
			t.Errorf("Incorrect output -- got %s - want %s", got, table.output)
		}
	}
}

func TestGetRule(t *testing.T) {
	// Clear and import rules
	rules = make(map[string][]string)
	err := importRules(false)
	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	tables := []struct {
		input  string
		output string
	}{
		{"100.1a", "\x02100.1a.\x0F A two-player game is a game that begins with only two players."},
		{"r 100.1a", "\x02100.1a.\x0F A two-player game is a game that begins with only two players."},
		{"cr 100.1a", "\x02100.1a.\x0F A two-player game is a game that begins with only two players."},
		{"rule 100.1a", "\x02100.1a.\x0F A two-player game is a game that begins with only two players."},
		{"def Absorb", "\x02Absorb\x0F: A keyword ability that prevents damage. See rule 702.63, \"Absorb.\""},
		{"define Absorb", "\x02Absorb\x0F: A keyword ability that prevents damage. See rule 702.63, \"Absorb.\""},
		{"rule Absorb", "\x02Absorb\x0F: A keyword ability that prevents damage. See rule 702.63, \"Absorb.\""},
		{"ex 101.2", "\x02[101.2] Example:\x0F If one effect reads \"You may play an additional land this turn\" and another reads \"You can’t play lands this turn,\" the effect that precludes you from playing lands wins."},
		{"ex101.2", "\x02[101.2] Example:\x0F If one effect reads \"You may play an additional land this turn\" and another reads \"You can’t play lands this turn,\" the effect that precludes you from playing lands wins."},
		{"example 101.2", "\x02[101.2] Example:\x0F If one effect reads \"You may play an additional land this turn\" and another reads \"You can’t play lands this turn,\" the effect that precludes you from playing lands wins."},
		{"ex 999.99", "Example not found"},
		{"example 999.99", "Example not found"},
		{"999.99", "Rule not found"},
		{"r 999.99", "Rule not found"},
		{"cr 999.99", "Rule not found"},
		{"rule 999.99", "Rule not found"},
		{"def CLOWNS", ""},
		{"define CLOWNS", ""},
		{"define adsorb", "\x02Absorb\x0F: A keyword ability that prevents damage. See rule 702.63, \"Absorb.\""},
		{"define deaftouch", "\x02Deathtouch\x0F: A keyword ability that causes damage dealt by an object to be especially effective. See rule 702.2, \"Deathtouch.\""},
		{"define die", "\x02Dies\x0F: A creature or planeswalker \"dies\" if it is put into a graveyard from the battlefield. See rule 700.4."},
	}
	for _, table := range tables {
		got := handleRulesQuery(table.input)
		if got != table.output {
			t.Errorf("Incorrect output -- got %s - want %s", got, table.output)
		}
	}
}

func TestCardMetadata(t *testing.T) {
	tables := []struct {
		command string
		message string
		output  string
	}{
		{"ruling", "ruling TestCardWithOneWOTCRuling 1", "1900-01-01: Print Me"},
		{"ruling", "ruling TestCardWithOneWOTCRuling", "1900-01-01: Print Me"},
		{"ruling", "ruling TestCardWithOneNonWOTCRuling 1", "Ruling not found"},
		{"ruling", "ruling TestCardWithOneNonWOTCRuling", "Ruling not found"},
		{"reminder", "reminder Ponder", "Reminder text not found"},
		{"reminder", "reminder Faithless Looting", "You may cast this card from your graveyard for its flashback cost. Then exile it."},
		{"reminder", "reminder Poison-Tip Archer", "This creature can block creatures with flying.\nAny amount of damage this deals to a creature is enough to destroy it."},
		{"flavour", "flavour Ponder", "Tomorrow belongs to those who prepare for it today."},
		{"flavor", "flavor Faithless Looting", "\"Avacyn has abandoned us! We have nothing left except what we can take!\""},
		{"flavor", "flavor Bushi Tenderfoot", "Flavour text not found"},
	}
	for _, table := range tables {
		got := handleCardMetadataQuery(table.command, table.message, fakeGetCard)
		if got != table.output {
			t.Errorf("Incorrect output -- got %s - want %s", got, table.output)
		}
	}
}

func TestHelp(t *testing.T) {
	got := printHelp()
	if !strings.Contains(got, "!cardname") {
		t.Errorf("Incorrect output -- got %s - want %s", got, "!cardname")
	}
}
