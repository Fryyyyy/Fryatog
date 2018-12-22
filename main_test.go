package main

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func fakeGetCard(cardname string) (string, error) {
	r := rand.Intn(1000)
	fmt.Printf("Trying to get card %v -- Sleeping %v ms\n", cardname, r)
	time.Sleep(time.Duration(r) * time.Millisecond)
	return "CARD", nil
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
	err := fetchOrImportRules()
	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	var emptyStringSlice []string
	tables := []struct {
		input  string
		output []string
	}{
		{"Hello! ", []string{}},
		{"<Bird12> Just making sure, thank you!!!!", emptyStringSlice},
		{"<Cyclops7> Thank you!! I have one more question kind of in the same realm-- if I want to bring some tokens with me to the same event, am I allowed to keep them in the deckbox with my deck and sideboard, or do I have to keep them someplace else?", []string{}},
		{"<+mtgrelay> [Fear12] Hi!! Quick question: Does Sundial of the Infinite bypass/combo with Psychic Vortex?", []string{}},
		{"<+mtgrelay> [Fear12] Hi!! Quick question: Does !Sundial of the Infinite bypass/combo with !Psychic Vortex?", []string{"CARD", "CARD"}},
		{"<MW> !!fract ident &treas nabb", []string{"CARD", "CARD"}},
		{"!cr 100.1a", []string{"A two-player game is a game that begins with only two players."}},
		{"!100.1a !!hi", []string{"A two-player game is a game that begins with only two players.", "CARD"}},
	}
	for _, table := range tables {
		got := tokeniseAndDispatchInput(table.input, fakeGetCard)
		if !reflect.DeepEqual(got, table.output) {
			t.Errorf("Incorrect output for [%v] -- got %s -- want %s", table.input, got, table.output)
		}
	}
}

func TestRules(t *testing.T) {
	err := fetchOrImportRules()
	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	tables := []struct {
		input  string
		output []string
	}{
		{"100.1a", []string{"A two-player game is a game that begins with only two players."}},
		{"Absorb", []string{`A keyword ability that prevents damage. See rule 702.63, "Absorb."`}},
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
	err := fetchOrImportRules()
	if err != nil {
		t.Errorf("Didn't expect an error -- got %v", err)
	}
	tables := []struct {
		input  string
		output string
	}{
		{"100.1a", "A two-player game is a game that begins with only two players."},
		{"r 100.1a", "A two-player game is a game that begins with only two players."},
		{"cr 100.1a", "A two-player game is a game that begins with only two players."},
		{"rule 100.1a", "A two-player game is a game that begins with only two players."},
		{"def Absorb", `A keyword ability that prevents damage. See rule 702.63, "Absorb."`},
		{"define Absorb", `A keyword ability that prevents damage. See rule 702.63, "Absorb."`},
		{"ex 101.2", `Example: If one effect reads "You may play an additional land this turn" and another reads "You can’t play lands this turn," the effect that precludes you from playing lands wins.`},
		{"example 101.2", `Example: If one effect reads "You may play an additional land this turn" and another reads "You can’t play lands this turn," the effect that precludes you from playing lands wins.`},
		{"ex 999.99", ""},
		{"example 999.99", ""},
		{"999.99", ""},
		{"r 999.99", ""},
		{"cr 999.99", ""},
		{"rule 999.99", ""},
		{"def CLOWNS", ""},
		{"define CLOWNS", ""},
	}
	for _, table := range tables {
		got := handleRulesQuery(table.input)
		if got != table.output {
			t.Errorf("Incorrect output -- got %s - want %s", got, table.output)
		}
	}
}
