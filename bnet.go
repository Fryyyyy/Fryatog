package main

import (
	"fmt"
	"strings"

	"github.com/FuzzyStatic/blizzard/wowp"
	raven "github.com/getsentry/raven-go"
)

//TODO: wowchar
//TODO: wow

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

func retrieveChievesForPlayer(realm, player string) (*wowp.CharacterAchievementsSummary, error) {
	if c, ok := wowPlayerChieveCache.Get(realm + "-" + player); ok {
		return c.(*wowp.CharacterAchievementsSummary), nil
	}
	cas, _, err := bNetClient.WoWCharacterAchievementsSummary(realm, player)
	if err != nil {
		return nil, err
	}
	wowPlayerChieveCache.SetDefault(realm+"-"+player, cas)
	return cas, nil
}

func chieveForPlayer(input1, input2, chieveName string) string {
	if bNetClient == nil {
		return "WOW API not available"
	}
	realm, player, err := distinguishRealmFromPlayer(input1, input2)
	if err != nil {
		return "Could not distinguish realm"
	}
	cas, err := retrieveChievesForPlayer(realm, player)
	if err != nil {
		raven.CaptureError(err, nil)
		return "Could not retrieve Chieves for Player"
	}
	for _, a := range cas.Achievements {
		if a.Achievement.Name == chieveName {
			if a.Criteria.IsCompleted {
				return ":fire: Achievement Unlocked! :fire:"
			}
			if len(a.Criteria.ChildCriteria) > 0 {
				var ret []string
				return strings.Join(ret, "\n")
			}
			return ":cry: Chievo not got :cry:"
		}
	}
	return "Chieve not found"
}

func retrieveChieves() error {
	var err error
	if wowChieves == nil {
		wowChieves, _, err = bNetClient.WoWAchievementIndex()
		if err != nil {
			raven.CaptureError(err, nil)
			return err
		}
	}
	return nil
}

func chieveDetails(chieveName string) string {
	if bNetClient == nil {
		return "WOW API not available"
	}
	retrieveChieves()
	var chieveToGet int
	for _, a := range wowChieves.Achievements {
		if strings.ToLower(a.Name) == strings.ToLower(chieveName) {
			chieveToGet = a.ID
		}
	}
	if chieveToGet == 0 {
		return "Chieve not found"
	}
	return formatChieve(chieveToGet)
}

func formatChieve(chieveID int) string {
	c, _, err := bNetClient.WoWAchievement(chieveID)
	if err != nil {
		raven.CaptureError(err, nil)
		return "Unable to fetch Chieve from API"
	}
	return c.Name
}

type wowDude struct {
	cps *wowp.CharacterProfileSummary
	css *wowp.CharacterStatisticsSummary
	ces *wowp.CharacterEquipmentSummary
}

/*
func retrieveDude(player, realm string) (wowDude, error) {
	if c, ok := wowPlayerCache.Get(realm + "-" + player); ok {
		return c.(wowDude), nil
	}
	aps, _, err := bNetClient.WoWCharacterProfileSummary(realm, player)
	if err != nil {
		raven.CaptureError(err, nil)
		return nil, err
	}
	return aps, nil
}

func getWoWDude(input1, input2 string) string {
	if bNetClient == nil {
		return "WOW API not available"
	}
	realm, player, err := distinguishRealmFromPlayer(input1, input2)
	if err != nil {
		return "Could not distinguish realm"
	}
	aps, err := retrieveDude(player, realm)
	if err != nil {
		return "Problem retrieving player"
	}

}
*/
