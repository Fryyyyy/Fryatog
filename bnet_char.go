package main

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/FuzzyStatic/blizzard/v2/wowp"
	raven "github.com/getsentry/raven-go"
	log "gopkg.in/inconshreveable/log15.v2"
)

type wowDude struct {
	cps *wowp.CharacterProfileSummary
	css *wowp.CharacterStatisticsSummary
	ces *wowp.CharacterEquipmentSummary
	cas *wowp.CharacterAchievementsStatistics
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
	cr, _, err := bNetClient.WoWCharacterRaids(ctx, realm, player)
	if err != nil {
		log.Warn("GDR", "Err", err)
		raven.CaptureError(err, nil)
		return "Could not retrieve raids"
	}
	for _, ex := range cr.Expansions {
		if strings.EqualFold(strings.ToLower(ex.Expansion.Name), strings.ToLower(expn)) {
			for _, i := range ex.Instances {
				if strings.EqualFold(strings.ToLower(i.Instance.Name), strings.ToLower(tier)) {
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
	cps, _, err := bNetClient.WoWCharacterProfileSummary(ctx, realm, player)
	if err != nil {
		log.Warn("RD", "CPS", err)
		raven.CaptureError(err, nil)
		return ret, err
	}
	css, _, err := bNetClient.WoWCharacterStatisticsSummary(ctx, realm, player)
	if err != nil {
		log.Warn("RD", "CSS", err)
		raven.CaptureError(err, nil)
		return ret, err
	}
	ces, _, err := bNetClient.WoWCharacterEquipmentSummary(ctx, realm, player)
	if err != nil {
		log.Warn("RD", "CES", err)
		raven.CaptureError(err, nil)
		return ret, err
	}
	cas, _, err := bNetClient.WoWCharacterAchievementsStatistics(ctx, realm, player)
	if err != nil {
		log.Warn("RD", "CAS", err)
		raven.CaptureError(err, nil)
		return ret, err
	}
	ret = wowDude{
		cps: cps,
		css: css,
		ces: ces,
		cas: cas,
	}
	return ret, nil
}

func printWoWDude(input1, input2 string) string {
	log.Debug("PWD", "Player", input1, "Realm", input2)
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

func handleStatInput(input string) string {
	tokens := strings.SplitN(input, " ", 3)
	log.Debug("Handling Stat Input", "Input", input, "Tokens", tokens)
	realm, player, err := distinguishRealmFromPlayer(tokens[0], tokens[1])
	if err != nil {
		return err.Error()
	}
	var statName string
	if len(tokens) < 3 {
		statName = "random"
	} else {
		statName = tokens[2]
	}
	p, err := retrieveDude(player, realm)
	if err != nil {
		return "Problem retrieving player"
	}
	statName, statDesc, statQty, err := getDudeStat(p, statName)
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%s : %s (%v)", statName, statDesc, statQty)
}

func handleStatFightInput(input string) string {
	tokens := strings.SplitN(input, " ", 5)
	if len(tokens) < 4 {
		return "Invalid command"
	}
	var statName string
	if len(tokens) == 4 {
		statName = "random"
	} else {
		statName = tokens[4]
	}
	log.Debug("Handling Stat Fight Input", "Input", input, "Tokens", tokens)
	if bNetClient == nil {
		return "WOW API not available"
	}
	realm1, player1, err := distinguishRealmFromPlayer(tokens[0], tokens[1])
	if err != nil {
		return "Could not distinguish realm for Player 1"
	}
	p1, err := retrieveDude(player1, realm1)
	if err != nil {
		return "Problem retrieving player 1"
	}
	realm2, player2, err := distinguishRealmFromPlayer(tokens[2], tokens[3])
	if err != nil {
		return "Could not distinguish realm for Player 2"
	}
	p2, err := retrieveDude(player2, realm2)
	if err != nil {
		return "Problem retrieving player 2"
	}
	return statFight(p1, p2, statName)
}

func statFight(p1, p2 wowDude, statName string) string {
	log.Debug("StatFight", "p1", p1.cps.Name, "p2", p2.cps.Name, "Name", statName)
	if statName == "random" {
		p1stats := populateWoWStats(p1)
		p2stats := populateWoWStats(p2)
		var commonStats []string
		for _, s := range p1stats {
			if stringSliceContains(p2stats, s) {
				commonStats = append(commonStats, s)
			}
		}
		statName = commonStats[rand.Intn(len(commonStats))]
		log.Debug("StatFight Randomised", "p1 Stat Len", len(p1stats), "p2 Stat Len ", len(p2stats), "Common len", len(commonStats), "Name", statName)
	}
	p1name, p1desc, p1qty, err := getDudeStat(p1, statName)
	if err != nil {
		return err.Error()
	}
	_, p2desc, p2qty, err := getDudeStat(p2, p1name)
	if err != nil {
		return err.Error()
	}
	if p1qty > p2qty {
		return fmt.Sprintf("%s\n[:white_check_mark:] %s : %s (%v) vs %s : %s (%v) [:x:]", statName, p1.cps.Name, p1desc, p1qty, p2.cps.Name, p2desc, p2qty)
	} else if p1qty < p2qty {
		return fmt.Sprintf("%s\n[:x:] %s : %s (%v) vs %s : %s (%v) [:white_check_mark:]", p1name, p1.cps.Name, p1desc, p1qty, p2.cps.Name, p2desc, p2qty)
	} else {
		return fmt.Sprintf("%s\n[:interrobang:] TIE!! Both on %v", p1name, p1qty)
	}
}

// Returns Stat Name, Stat Description, Stat Quantity and Error
func getDudeStat(player wowDude, statName string) (string, string, float64, error) {
	log.Debug("Get stat", "Player", player.cps.Name, "Stat", statName)
	if bNetClient == nil {
		return "", "", 0, fmt.Errorf("WOW API not available")
	}
	if statName == "random" {
		stats := populateWoWStats(player)
		statName = stats[rand.Intn(len(stats))]
	}
	for _, cat := range player.cas.Categories {
		for _, sc := range cat.SubCategories {
			for _, stat := range sc.Statistics {
				if strings.EqualFold(strings.ToLower(stat.Name), strings.ToLower(statName)) {
					return stat.Name, stat.Description, stat.Quantity, nil
				}
			}
		}
		for _, stat := range cat.Statistics {
			if strings.EqualFold(strings.ToLower(stat.Name), strings.ToLower(statName)) {
				return stat.Name, stat.Description, stat.Quantity, nil
			}
		}
	}
	return "", "", 0, fmt.Errorf("Stat not found")
}

func getDudeReps(input1, input2 string) string {
	log.Debug("Get reps", "Player", input1, "Realm", input2)
	if bNetClient == nil {
		return "WOW API not available"
	}
	realm, player, err := distinguishRealmFromPlayer(input1, input2)
	if err != nil {
		return "Could not distinguish realm"
	}
	var ret []string
	reps, _, err := bNetClient.WoWCharacterReputationsSummary(ctx, realm, player)
	if err != nil {
		log.Warn("GDRep", "Err", err)
		raven.CaptureError(err, nil)
		return "Could not retrieve reputations"
	}
	for _, r := range reps.Reputations {
		if stringSliceContains(conf.BattleNet.Reputations, r.Faction.Name) {
			emoji := ":question_man:"
			switch r.Standing.Name {
			case "Hated":
				emoji = ":angry:"
			case "Stranger":
				emoji = ":thinking_tom:"
			case "Neutral":
				emoji = ":neutral_face:"
			case "Friendly":
				emoji = ":fry_real:"
			case "Honored":
				emoji = ":ok_hand:"
			case "Revered":
				emoji = ":bflove:"
			case "Exalted":
				emoji = ":angrylaugh:"
			}
			ret = append(ret, fmt.Sprintf("%s %s - %s [%d/%d]", emoji, r.Faction.Name, r.Standing.Name, r.Standing.Value, r.Standing.Max))
		}
	}
	return strings.Join(ret, "\n")
}
