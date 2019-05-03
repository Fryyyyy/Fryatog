package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	charmap "golang.org/x/text/encoding/charmap"
	log "gopkg.in/inconshreveable/log15.v2"
)

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
	r := charmap.Windows1252.NewDecoder().Reader(f)
	scanner := bufio.NewScanner(r)
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
	for scanner.Scan() {
		line := scanner.Text()
		if rulesMode && line == "" {
			continue
		}
		// Clean up line
		line = strings.Replace(line, "“", `"`, -1)
		line = strings.Replace(line, "”", `"`, -1)
		line = strings.Replace(line, "’", `'`, -1)
		// "Glossary" in the T.O.C
		if line == "Glossary" {
			if !metGlossary {
				metGlossary = true
			} else {
				// Done with the rules, let's start Glossary mode.
				rulesMode = false
			}
		} else if line == "Credits" {
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
			} else {
				// log.Debug("In scanner", "Rules mode: Ignored line", line)
			}
		} else {
			if line == "" {
				lastGlossary = ""
			} else if lastGlossary != "" {
				rules[lastGlossary] = append(rules[lastGlossary], fmt.Sprintf("\x02%s\x0F: %s", lastGlossary, line))
			} else {
				lastGlossary = line
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
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
