package main

import (
	"fmt"
	"strings"

	"github.com/FuzzyStatic/blizzard/wowgd"
)

const corruptionR = 149
const corruptionG = 109
const corruptionB = 209

/* REALM UTILITIES */
func makeRealmList(ri *wowgd.RealmIndex) []string {
	var rl []string
	for _, r := range ri.Realms {
		rl = append(rl, r.Name)
	}
	return rl
}

// Given two strings, if either one is the name of a realm (case insensitive)
// returns realm, player (, error).
func distinguishRealmFromPlayer(input1, input2 string) (string, string, error) {
	for _, r := range wowRealms.Realms {
		// Exact match
		if strings.ToLower(input1) == strings.ToLower(r.Name) {
			return r.Slug, input2, nil
		}
		if strings.ToLower(input2) == strings.ToLower(r.Name) {
			return r.Slug, input1, nil
		}
	}
	return "", "", fmt.Errorf("Realm not found")
}
