package main

import (
	"fmt"
	"strings"
)

const corruptionR = 149
const corruptionG = 109
const corruptionB = 209

/* REALM UTILITIES */

// Given two strings, if either one is the name of a realm (case insensitive)
// returns realm, player (, error).
func distinguishRealmFromPlayer(input1, input2 string) (string, string, error) {
	for _, r := range wowRealms.Realms {
		// Exact match
		if strings.EqualFold(strings.ToLower(input1), strings.ToLower(r.Name)) {
			return r.Slug, input2, nil
		}
		if strings.EqualFold(strings.ToLower(input2), strings.ToLower(r.Name)) {
			return r.Slug, input1, nil
		}
	}
	return "", "", fmt.Errorf("Realm not found")
}

// Given a WowDude with stats, give back all the available stat names.
func populateWoWStats(wd wowDude) []string {
	var ret []string
	for _, cat := range wd.cas.Categories {
		for _, sc := range cat.SubCategories {
			for _, stat := range sc.Statistics {
				ret = append(ret, stat.Name)
			}
		}
		for _, stat := range cat.Statistics {
			ret = append(ret, stat.Name)
		}
	}
	return ret
}
