package main

import (
	"fmt"
	"strings"

	"github.com/FuzzyStatic/blizzard/wowp"
	raven "github.com/getsentry/raven-go"
	log "gopkg.in/inconshreveable/log15.v2"
)

type wowDude struct {
	cps *wowp.CharacterProfileSummary
	css *wowp.CharacterStatisticsSummary
	ces *wowp.CharacterEquipmentSummary
}

func getDudeRaid(input1, input2, expn, tier string) string {
	log.Debug("GDR", "Player", input1, "Realm", input2, "Expn", expn, "Tier", tier)
	if bNetClient == nil {
		return "WOW API not available"
	}
	realm, player, err := distinguishRealmFromPlayer(input1, input2)
	if err != nil {
		return "Could not distinguish realm"
	}
	var ret []string
	cr, _, err := bNetClient.WoWCharacterRaids(realm, player)
	for _, ex := range cr.Expansions {
		if strings.ToLower(ex.Expansion.Name) == strings.ToLower(expn) {
			for _, i := range ex.Instances {
				if strings.ToLower(i.Instance.Name) == strings.ToLower(tier) {
					ret = append(ret, fmt.Sprintf("%s - %s", ex.Expansion.Name, i.Instance.Name))
					for _, m := range i.Modes {
						t := ""
						if m.Status.Name == "Complete" {
							t = ":trophy: "
						}
						ret = append(ret, fmt.Sprintf("Difficulty: %s -- %s%d/%d", m.Difficulty.Name, t, m.Progress.CompletedCount, m.Progress.TotalCount))
					}
					return strings.Join(ret, "\n")
				}
			}
			return "Raid found, instance not found"
		}
	}
	return "Raid not found"
}

func retrieveDude(player, realm string) (wowDude, error) {
	log.Debug("RD", "Player", player, "Realm", realm)
	if c, ok := wowPlayerCache.Get(realm + "-" + player); ok {
		return c.(wowDude), nil
	}
	var ret wowDude
	cps, _, err := bNetClient.WoWCharacterProfileSummary(realm, player)
	if err != nil {
		log.Warn("RD", "CPS", err)
		raven.CaptureError(err, nil)
		return ret, err
	}
	css, _, err := bNetClient.WoWCharacterStatisticsSummary(realm, player)
	if err != nil {
		log.Warn("RD", "CSS", err)
		raven.CaptureError(err, nil)
		return ret, err
	}
	ces, _, err := bNetClient.WoWCharacterEquipmentSummary(realm, player)
	if err != nil {
		log.Warn("RD", "CES", err)
		raven.CaptureError(err, nil)
		return ret, err
	}
	ret = wowDude{
		cps: cps,
		css: css,
		ces: ces,
	}
	return ret, nil
}

func printWoWDude(input1, input2 string) string {
	if bNetClient == nil {
		return "WOW API not available"
	}
	realm, player, err := distinguishRealmFromPlayer(input1, input2)
	if err != nil {
		return "Could not distinguish realm"
	}
	wd, err := retrieveDude(player, realm)
	if err != nil {
		return "Problem retrieving player"
	}
	var ret []string
	var name = wd.cps.Name
	if len(wd.cps.ActiveTitle.DisplayString) > 0 {
		name = strings.Replace(wd.cps.ActiveTitle.DisplayString, "{name}", wd.cps.Name, -1)
	}
	ret = append(ret, fmt.Sprintf("%s - Level %d %s %s", name, wd.cps.Level, wd.cps.ActiveSpec.Name, wd.cps.CharacterClass.Name))
	ret = append(ret, fmt.Sprintf("iLvl %d -- :trophy: %d", wd.cps.AverageItemLevel, wd.cps.AchievementPoints))
	for _, ei := range wd.ces.EquippedItems {
		emoji := fmt.Sprintf(":wow-%s:", strings.ToLower(string(ei.Quality.Name[0])))
		sText := ""
		for _, s := range ei.Spells {
			// Corruption purple
			if s.DisplayColor.R == corruptionR && s.DisplayColor.G == corruptionG && s.DisplayColor.B == corruptionB {
				sText = "[Corruption: " + s.Spell.Name + "]"
			}
		}
		ret = append(ret, fmt.Sprintf("%s <http://www.wowhead.com/item=%d|%s> (%d) %s", emoji, ei.Item.ID, ei.Name, ei.Level.Value, sText))
		// TODO: Context (i.e Mythic, WQ, etc)
	}
	return strings.Join(ret, "\n")
}
