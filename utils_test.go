package main

import (
	"testing"
	"strings"
)

func TestGreeting(t *testing.T) {
	tables := []struct {
		input  string
		output bool
	}{
		{"Hello!", true},
		{"Hello", true},
		{"Hello       ?", true},
		{"Hello! I have a question.", false},
		{"Hello I have a question.", false},
		{"Hi, I have a question.", false},
		{"Hello!!!!!!!!", true},
		{"Hi??", true},
	}
	for _, table := range tables {
		match := greetingRegexp.MatchString(table.input)
		if match != table.output {
			t.Errorf("Incorrect result, want %v got %v", table.output, match)
		}
	}
}

func TestICC(t *testing.T) {
	tables := []struct {
		input  string
		output string
	}{
		{"ice crown citadel", "Ice Crown Citadel"},
		{"a b c ", "Ace Brown Citadel"},
	}
	for _, table := range tables {
		cardTokens := strings.Fields(table.input)
		match := handleICC(cardTokens)
		if match != table.output {
			t.Errorf("Incorrect result, want %v got %v", table.output, match)
		}
	}
}