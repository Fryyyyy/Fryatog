package main

import "testing"

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
