package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestFlipCoin(t *testing.T) {
	validOutput := regexp.MustCompile(`(?:Heads|Tails).`)
	for i := 1; i < 10; i++ {
		result := flipCoin("coin")
		if !(validOutput.MatchString(result)) {
			t.Errorf(`FAIL: Expected heads or tails in output, but result was \"%s\"`, result)
		}
	}

	validOutput = regexp.MustCompile(`(?:(?:Heads|Tails), ){2}(?:Heads|Tails)\.`)
	for i := 1; i < 10; i++ {
		result := flipCoin("coin 3")
		if !(validOutput.MatchString(result)) {
			t.Errorf(`FAIL: Expected list of heads or tails in output, but result was \"%s\"`, result)
		}
	}

	validOutput = regexp.MustCompile(`10 coins: [HT]{10}\.`)
	for i := 1; i < 10; i++ {
		result := flipCoin("coin 10")
		if !(validOutput.MatchString(result)) {
			t.Errorf(`FAIL: Expected list of H or T in output, but result was \"%s\"`, result)
		}
	}

	errorMsg := "malformed coin toss (max count is 50)"
	result := flipCoin("coin 99")
	if result != errorMsg {
		t.Errorf(`FAIL: Expected count exceeded error, got "%s"`, result)
	}

	errorMsg = "malformed coin toss (min count is 1)"
	result = flipCoin("coin 0")
	if result != errorMsg {
		t.Errorf(`FAIL: Expected count subceeded error, got "%s"`, result)
	}
}

func TestRollDice(t *testing.T) {
	plannedFailure := "Try something like '!roll d4', '!roll 3d8', '!roll 2d6+2'"
	plannedError := "malformed roll"
	errorTables := []struct {
		input    string
		expected string
	}{
		{"", plannedFailure},
		{" ", plannedFailure},
		{"4", plannedFailure},
		{"2d6+_", plannedFailure},
		{"2daf3", plannedFailure},
		{"2d6+e", plannedFailure},
		{"2e6?_", plannedFailure},
		{"-3d8", plannedFailure},
		{"3d1", plannedError},
		{"2d112", plannedError},
		{"4d0", plannedError},
		{"100d12", plannedError},
		{"4d8+1200", plannedError},
		{"92233720368547758072131231", plannedFailure},
		{"4d92233720368547758073213123", plannedError},
	}

	validateOutput := regexp.MustCompile(`\d+ \d+-sided dic?e(?::|\s(?:\([+-]\d\))?:) \d+`)
	successTables := []struct {
		input string
	}{
		{"d6"},
		{"4d6"},
		{"2d20+3"},
		{"2d6-2"},
		{"50d8"},
	}

	for _, table := range errorTables {
		result := rollDice(table.input)
		if !(strings.Contains(result, table.expected)) {
			t.Errorf("FAIL: Input %s: Expected %s -- Got %s", table.input, table.expected, result)
		} else {
			t.Logf("OK: %s", table.input)
		}
	}

	for _, table := range successTables {
		result := rollDice(table.input)
		if !(validateOutput.MatchString(result)) {
			t.Errorf("FAIL: Input %s: Got %s", table.input, result)
		} else {
			t.Logf("OK: %s -> %s", table.input, result)
		}
	}
}
