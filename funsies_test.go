package main

import (
	"regexp"
	"testing"
)

func TestFlipCoin(t *testing.T) {

	for i := 1; i < 10; i++ {
		result := FlipCoin("testUser")
		validOutput := regexp.MustCompile(`testUser flips a coin: (?:heads|tails).`)
		if !(validOutput.MatchString(result)) {
			t.Errorf(`FAIL: Expected heads or tails in output,
                      but result was \"%s\"`, result)
		}
	}
}

func TestRollDice(t *testing.T) {

	plannedFailure := "roll: Try something like 'roll 4', 'roll 3d8', 'roll 2d6+2'"
	tables := []struct {
		input    string
		expected string
	}{
		{"", plannedFailure},
		{" ", plannedFailure},
		{"kd4", plannedFailure},
		{"2d6+_", plannedFailure},
		{"2daf3", plannedFailure},
		{"2d6+e", plannedFailure},
		{"2e6?_", plannedFailure},
		{"-3d8", plannedFailure},
		{"3d1", "Your spherical dice go careening off the flat earth. You know. Those two things that exist."},
		{"2d112", "When have you ever seen a die with that many sides? Come on."},
		{"4d0", "Your spherical dice go careening off the flat earth. You know. Those two things that exist."},
		{"100d12", "That's too many dice. Calm down."},
		{"4d8+1200", "Haha. No. Why do you need a mod that large?"},
		{"92233720368547758072131231", "That's too many dice. Calm down."},
		{"4d92233720368547758073213123", "When have you ever seen a die with that many sides? Come on."},
	}

	validateOutput := regexp.MustCompile(`\d+ \d+-sided dic?e(?::|\s(?:\([+-]\d\))?:) \d+`)
	randTables := []struct {
		input string
	}{

		{"4"},
		{"4d6"},
		{"2d20+3"},
		{"2d6-2"},
		{"50d8"},
	}

	for _, table := range tables {
		result := RollDice(table.input)
		if !(result == table.expected) {
			t.Errorf("FAIL: Input %s: Expected %s -- Got %s", table.input, table.expected, result)
		} else {
			t.Logf("OK: %s", table.input)
		}
	}

	for _, table := range randTables {
		result := RollDice(table.input)
		if !(validateOutput.MatchString(result)) {
			t.Errorf("FAIL: Input %s: Got %s", table.input, result)
		} else {
			t.Logf("OK: %s -> %s", table.input, result)
		}
	}
}
