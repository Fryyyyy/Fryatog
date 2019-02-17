package main

import (
    "strings"
    "testing"
)

func TestFlipCoin(t *testing.T) {
    
    for i := 1; i < 10; i++ {
        result := FlipCoin("testUser")
        if !(strings.Contains(result, "heads") || strings.Contains(result, "tails")) {
            t.Errorf(`FAIL: Expected heads or tails in output,
                      but result was \"%s\"`, result)
        }
    }
}

func TestRollDice(t *testing.T) {
    tables := []struct {
        input string
        expected string
    } {
        {"", "\x02roll\x0F: Try something like 'roll 4', 'roll 3d8', 'roll 2d6+2'"},
        {" ", "\x02roll\x0F: Try something like 'roll 4', 'roll 3d8', 'roll 2d6+2'"},
        {"kd4", "\x02roll\x0F: Try something like 'roll 4', 'roll 3d8', 'roll 2d6+2'"},
        {"2d6+_", "\x02roll\x0F: Try something like 'roll 4', 'roll 3d8', 'roll 2d6+2'"},
        {"2daf3", "\x02roll\x0F: Try something like 'roll 4', 'roll 3d8', 'roll 2d6+2'"},
        {"2d6+e", "\x02roll\x0F: Try something like 'roll 4', 'roll 3d8', 'roll 2d6+2'"},
        {"2e6?_", "\x02roll\x0F: Try something like 'roll 4', 'roll 3d8', 'roll 2d6+2'"},
        {"4", "1 4-sided die: 3"},
        {"4d6", "4 6-sided dice : 13"},
        {"2d20+3", "2 20-sided dice +3: 24"},
        {"2d6-2", "2 6-sided dice -2: 3"},
        {"3d1", "Your spherical dice go careening off the flat earth. You know. Those two things that exist."},
    }

    for _, table := range tables {
        result := RollDice(table.input)
        if (result != table.expected) {
            t.Errorf("FAIL: Input %s: Expected %s -- Got %s", table.input, table.expected, result)
        } else {
            t.Logf("OK: %s", table.input)
        }
    }
}