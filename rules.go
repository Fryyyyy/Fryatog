package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	raven "github.com/getsentry/raven-go"
	fuzzy "github.com/paul-mannino/go-fuzzywuzzy"
	log "gopkg.in/inconshreveable/log15.v2"
)

// AbilityWord stores a quick description of Ability Words, which have no inherent rules meaning
type AbilityWord struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func importAbilityWords() error {
	log.Debug("In importAbilityWords")
	content, err := ioutil.ReadFile(abilityWordFile)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("Error opening abilityWords file", "Error", err)
		return err
	}
	var tempAbilityWords []AbilityWord
	err = json.Unmarshal(content, &tempAbilityWords)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("Unable to parse abilityWords file", "Error", err)
		return err
	}
	for _, aw := range tempAbilityWords {
		abilityWords[aw.Name] = aw.Description
		abilityWordKeys = append(abilityWordKeys, aw.Name)
	}
	log.Debug("Populated abilityWords", "Length", len(abilityWords))
	return nil
}

func importRules(forceFetch bool) error {
	log.Debug("In importRules", "Force?", forceFetch)
	if forceFetch {
		if err := fetchRulesFile(); err != nil {
			log.Warn("Error fetching rules file", "Error", err)
			return err
		}
	}

	if _, err := os.Stat(crFile); err != nil {
		if err := fetchRulesFile(); err != nil {
			log.Warn("Error fetching rules file", "Error", err)
			return err
		}
	}

	// Parse it.
	f, err := os.Open(crFile)
	defer f.Close()
	if err != nil {
		return err
	}
	// WOTC doesn't serve UTF-8. 😒
	//r := charmap.Windows1252.NewDecoder().Reader(f)
	//scanner := bufio.NewScanner(f)

	// OR DOES IT
	reader := bufio.NewReader(f)
	var (
		metGlossary  bool
		metCredits   bool
		lastRule     string
		lastGlossary string
		rulesMode    = true
	)

	// Clear rules map
	rules = make(map[string][]string)

	// Begin rules parsing
	for {
		line, err := reader.ReadString('\r')
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
		line = strings.Replace(line, "\r", "", -1)
		line = strings.Replace(line, "\n", "", -1)
		if rulesMode && line == "" {
			continue
		}
		// Clean up line
		line = strings.Replace(line, "“", `"`, -1)
		line = strings.Replace(line, "”", `"`, -1)
		line = strings.Replace(line, "’", `'`, -1)
		// "Glossary" in the T.O.C
		if line == "Glossary" {
			// log.Debug("Glossary")
			if !metGlossary {
				metGlossary = true
			} else {
				// Done with the rules, let's start Glossary mode.
				rulesMode = false
			}
		} else if line == "Credits" {
			// log.Debug("Credits")
			if !metCredits {
				metCredits = true
			} else {
				// Done!
				for key := range rules {
					rulesKeys = append(rulesKeys, key)
				}
				return nil
			}
		} else if rulesMode {
			if ruleParseRegex.MatchString(line) {
				rm := ruleParseRegex.FindAllStringSubmatch(line, -1)
				// log.Debug("In scanner. Rules Mode: found rule", "Rule number", rm[0][0], "Rule name", rm[0][1])
				if _, ok := rules[rm[0][1]]; ok {
					log.Warn("In scanner", "Already had a rule!", line, "Existing rule", rules[rm[0][1]])
				}
				rules[rm[0][1]] = append(rules[rm[0][1]], rm[0][2])
				lastRule = rm[0][1]
			} else if strings.HasPrefix(line, "Example: ") {
				if lastRule != "" {
					rules["ex"+lastRule] = append(rules["ex"+lastRule], line)
				} else {
					log.Warn("In scanner", "Got example without rule", line)
				}
			} else if strings.HasPrefix(line, "     ") {
				log.Debug("In scanner", "Follow on rule?", line)
				if lastRule != "" {
					rules[lastRule] = append(rules[lastRule], " "+strings.TrimSpace(line))
				}
			} else {
				// log.Debug("In scanner", "Rules mode: Ignored line", line)
			}
		} else {
			// log.Debug("In scanner", "Glossary mode:", line)
			if line == "" {
				lastGlossary = ""
			} else if lastGlossary != "" {
				rules[strings.ToLower(lastGlossary)] = append(rules[strings.ToLower(lastGlossary)], fmt.Sprintf("\x02%s\x0F: %s", lastGlossary, line))
			} else {
				lastGlossary = line
			}
		}
	}
	return nil
}

func tryFindSeeMoreRule(input string) string {
	if strings.Contains(input, "See rule") && !strings.Contains(input, "See rules") && !strings.Contains(input, "and rule") {
		matches := seeRuleRegexp.FindAllStringSubmatch(input, -1)
		if len(matches) > 0 {
			return "\n" + handleRulesQuery(matches[0][1])
		}
	}
	return ""
}

func tryFindBetterAbilityRule(ruleText, ruleNumber string) (string, string) {
	var forceB bool
	// 0) Exceptions: Landwalk, Forecast, Vigilance, Banding
	switch ruleText {
	case "Banding":
		fallthrough
	case "Landwalk":
		subRuleCLabel := ruleNumber + "c"
		subRuleC, ok := rules[subRuleCLabel]
		if !ok {
			log.Debug("In tryFindBetterAbilityRule", "There is no subrule C")
			return ruleText, ruleNumber
		}
		subRuleCText := strings.Join(subRuleC, "")
		return subRuleCText, subRuleCLabel
	case "Forecast":
		fallthrough
	case "Vigilance":
		forceB = true
	}
	// 1) If subrule a contains means and ends in Step."), take subrule a. This covers Rampage and Bushido.
	subRuleALabel := ruleNumber + "a"
	subRuleBLabel := ruleNumber + "b"
	subRuleA, ok := rules[subRuleALabel]
	if !ok {
		log.Debug("In tryFindBetterAbilityRule", "There is no subrule A")
		return ruleText, ruleNumber
	}
	subRuleAText := strings.Join(subRuleA, "")
	if !forceB && strings.Contains(subRuleAText, "means") && strings.HasSuffix(subRuleAText, `Step.")`) {
		return subRuleAText, subRuleALabel
	}

	// 2) If subrule a ends in ability. we should take subrule b. This covers the majority of your static and evasion abilities, except Landwalk, which has a useless a and mentions being a static ability in b.
	if forceB || strings.HasSuffix(subRuleAText, "ability.") || strings.HasSuffix(subRuleAText, `Step.")`) {
		subRuleB, ok := rules[subRuleBLabel]
		if !ok {
			log.Debug("In tryFindBetterAbilityRule", "There is no subrule B")
			return subRuleAText, subRuleALabel
		}
		subRuleBText := strings.Join(subRuleB, "")
		return subRuleBText, subRuleBLabel
	}
	// 3) Otherwise, just take subrule a
	return subRuleAText, subRuleALabel
}

func handleRulesQuery(input string) string {
	log.Debug("In handleRulesQuery", "Input", input)
	// Match example first, for !ex101.a and !example 101.1a so the rule regexp doesn't eat it as a normal rule
	if (strings.HasPrefix(input, "ex") || strings.HasPrefix(input, "example ")) && ruleRegexp.MatchString(input) {
		foundRuleNum := ruleRegexp.FindAllStringSubmatch(input, -1)[0][1]
		log.Debug("In handleRulesQuery", "Example matched on", foundRuleNum)
		if _, ok := rules["ex"+foundRuleNum]; !ok {
			return "Example not found"
		}
		var formattedExample []string
		exampleNumber := "\x02[" + foundRuleNum + "] Example:\x0F "
		for _, e := range rules["ex"+foundRuleNum] {
			formattedExample = append(formattedExample, exampleNumber+e[9:]+"\n")
		}
		return strings.TrimSpace(strings.Join(formattedExample, ""))
	}
	// Then try normal rules
	if ruleRegexp.MatchString(input) {
		foundRuleNum := ruleRegexp.FindAllStringSubmatch(input, -1)[0][1]
		log.Debug("In handleRulesQuery", "Rules matched on", foundRuleNum)

		if _, ok := rules[foundRuleNum]; !ok {
			return "Rule not found"
		}

		ruleText := strings.Join(rules[foundRuleNum], "")

		// keyword abilities can just tag subrule a
		if foundKeywordAbilityRegexp.MatchString(input) {
			subRuleALabel := foundRuleNum + "a"
			subRuleA, ok := rules[subRuleALabel]
			if !ok {
				log.Debug("In 701 handler", "There is no subrule A")
			} else {
				foundRuleNum = subRuleALabel
				ruleText = strings.Join(subRuleA, "")
			}

		}

		// keyword actions need a little bit more work
		if foundKeywordActionRegexp.MatchString(input) {
			ruleText, foundRuleNum = tryFindBetterAbilityRule(ruleText, foundRuleNum)
		}
		ruleNumber := []string{"\x02", foundRuleNum, ".\x0F "}
		ruleWithNumber := append(ruleNumber, ruleText, "\n")
		return strings.TrimSpace(strings.Join(ruleWithNumber, ""))
	}
	// Finally try Glossary entries, people might do "!rule Deathtouch" rather than the proper "!define Deathtouch"
	if strings.HasPrefix(input, "def ") || strings.HasPrefix(input, "define ") || strings.HasPrefix(input, "rule ") || strings.HasPrefix(input, "r ") || strings.HasPrefix(input, "cr ") {
		split := strings.SplitN(input, " ", 2)
		log.Debug("In handleRulesQuery", "Define matched on", split)
		query := strings.ToLower(split[1])
		var defineText string
		v, ok := rules[query]
		if ok {
			log.Debug("Rules exact match")
			defineText = strings.Join(v, "\n")
		} else {
			a, ok := abilityWords[query]
			if ok {
				log.Debug("Ability word exact match")
				return "\x02" + strings.Title(query) + "\x0F: " + a
			}
			// Special case, otherwise it matches "Planar Die" better
			if query == "die" {
				query = "dies"
			}
			if bestGuess, err := fuzzy.ExtractOne(query, rulesKeys); err != nil {
				log.Info("InExact rules match", "Error", err)
			} else {
				log.Debug("InExact rules match", "Guess", bestGuess)
				if bestGuess.Score > 80 {
					defineText = strings.Join(rules[bestGuess.Match], "\n")
				}
			}
			if defineText == "" {
				if bestGuess, err := fuzzy.ExtractOne(query, abilityWordKeys); err != nil {
					log.Info("InExact aw match", "Error", err)
				} else {
					log.Debug("InExact aw match", "Guess", bestGuess)
					if bestGuess.Score > 80 {
						return "\x02" + strings.Title(bestGuess.Match) + "\x0F: " + abilityWords[bestGuess.Match]
					}
				}
			}
		}
		// Some crappy workaround/s
		if !strings.HasPrefix(defineText, "\x02Dies\x0F:") {
			defineText += tryFindSeeMoreRule(defineText)
		}
		return strings.TrimSpace(defineText)
	}
	// Didn't match ??
	return ""
}
