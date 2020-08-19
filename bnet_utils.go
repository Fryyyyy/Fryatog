package main

import (
	"fmt"
	"strings"

	"github.com/FuzzyStatic/blizzard/wowgd"
	raven "github.com/getsentry/raven-go"
)

/* REALM UTILITIES */

// Given two strings, if either one is the name of a realm (case insensitive)
// returns realm, player (, error).
func distinguishRealmFromPlayer(input1, input2 string) (string, string, error) {
	for _, r := range wowRealms.Realms {
		if strings.ToLower(input1) == strings.ToLower(r.Name) {
			return r.Slug, input2, nil
		}
		if strings.ToLower(input2) == strings.ToLower(r.Name) {
			return r.Slug, input1, nil
		}
	}
	return "", "", fmt.Errorf("Realm not found")
}

/* CHIEVE UTILITIES */
func handleChieveInput(input string) string {
	cardTokens := strings.SplitN(input, " ", 3)
	realm, player, err := distinguishRealmFromPlayer(cardTokens[0], cardTokens[1])
	if err != nil {
		return formatChieveForSlack(chieveFromID(chieveNameToID(input)))
	}
	return chieveForPlayer(realm, player, cardTokens[2])

}
func chieveNameToID(chieveName string) int {
	if bNetClient == nil || len(wowChieves.Achievements) == 0 {
		return 0
	}

	for _, a := range wowChieves.Achievements {
		if strings.ToLower(a.Name) == strings.ToLower(chieveName) {
			return a.ID
		}
	}

	// Not found
	return 0
}

// Little wrapper to make the format function hermetic.
func chieveFromID(chieveID int) *wowgd.Achievement {
	c, _, err := bNetClient.WoWAchievement(chieveID)
	if err != nil {
		raven.CaptureError(err, nil)
		return nil
	}
	return c
}

func dedupeCriteria(cs wowgd.ChildCriteria) wowgd.ChildCriteria {
	var ret wowgd.ChildCriteria
	keys := make(map[string]bool)
	for _, r := range cs {
		if _, ok := keys[r.Description]; !ok {
			keys[r.Description] = true
			ret = append(ret, r)
		}
	}
	return ret
}
