package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	raven "github.com/getsentry/raven-go"
	fuzzy "github.com/paul-mannino/go-fuzzywuzzy"
	log "gopkg.in/inconshreveable/log15.v2"
)

const voloRulesEndpointURL = "https://slack.vensersjournal.com/rule/"
const voloExamplesEndpointURL = "https://slack.vensersjournal.com/example/"
const voloSpecificRuleEndpointURL = "https://www.vensersjournal.com/"

var tooLongRules = []string{"205.3i", "205.3j", "205.3m", "205.3n"}

// AbilityWord stores a quick description of Ability Words, which have no inherent rules meaning
type AbilityWord struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Rule stores the result of Rules queries from Volo's API
type Rule struct {
	RuleNumber      string `json:"ruleNumber"`
	RuleText        string `json:"ruleText"`
	RawExampleTexts string `json:"exampleText"`
	ExampleTexts    []string
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
	if err != nil {
		return err
	}
	defer f.Close()

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
				// log.Debug("In scanner", "Follow on rule?", line)
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
				if strings.Contains(lastGlossary, "(Obsolete)") {
					// including the leading " " here so we don't end up having "thing " in the indices
					lastGlossary = strings.Replace(lastGlossary, " (Obsolete)", "", -1)
				}
				if strings.Contains(lastGlossary, ",") {
					gl := strings.Split(lastGlossary, ",")
					for _, g := range gl {
						rules[strings.ToLower(g)] = append(rules[strings.ToLower(g)], fmt.Sprintf("<b>%s</b>: %s", lastGlossary, line))
					}
				} else {
					rules[strings.ToLower(lastGlossary)] = append(rules[strings.ToLower(lastGlossary)], fmt.Sprintf("<b>%s</b>: %s", lastGlossary, line))
				}
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
		if strings.Contains(input, "Source of Damage") {
			return "\n" + handleRulesQuery(matches[0][1]+"a")
		}
		// Doing a couple things here:
		// First, we want to match mana ability/ies, but too narrow to bother with regex
		// Second, the rules reference in this definition DOES match our regex, so
		// I'd rather use that match instead of hardcore 605.1a (as of 31/12/19).
		if strings.Contains(input, "Mana Abilit") {
			return "\n" + handleRulesQuery(matches[0][1]+".1a")
		}

		if strings.Contains(input, "Monarch") {
			return "\n" + handleRulesQuery(matches[0][1]+".2")
		}

		if strings.Contains(input, "Destroy") {
			return "\n" + handleRulesQuery(matches[0][1]+"b")
		}

		if len(matches) > 0 {
			return "\n" + handleRulesQuery(matches[0][1])
		}
	}
	return ""
}

func findRule(input string, which string) (Rule, error) {
	var endpoint string
	switch which {
	case "example":
		endpoint = voloExamplesEndpointURL
	case "rule":
		endpoint = voloRulesEndpointURL
	default:
		log.Error("findRule", "Called with unknown mode", which)
		return Rule{}, errors.New("Unknown mode")
	}

	url := endpoint + input
	log.Debug("findRule: Attempting to fetch", "URL", url)
	resp, err := http.Get(url)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Debug("HTTP request to Volo Rules Endpoint failed", "Error", err)
		return Rule{}, err
	}
	defer resp.Body.Close()
	var foundRule Rule
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&foundRule); err != nil {
			raven.CaptureError(err, nil)
			log.Debug("Failed decoding the response", "Error", err)
			return Rule{}, err
		}
		return foundRule, nil
	}
	return Rule{}, errors.New("Whatever you requested failed")
}

func handleExampleQuery(input string) string {
	exampleRequests.Add(1)
	foundRuleNum := ruleRegexp.FindAllStringSubmatch(input, -1)[0][1]
	log.Debug("In handleExampleQuery (Volo)", "Example matched on", foundRuleNum)

	foundExample, err := findRule(foundRuleNum, "example")

	if err != nil || foundExample.RawExampleTexts == "" {
		return "Example not found"
	}

	foundExample.ExampleTexts = strings.Split(foundExample.RawExampleTexts, "\n")
	var formattedExample []string
	exampleNumber := "<b>[" + foundExample.RuleNumber + "] Example:</b> "

	for _, e := range foundExample.ExampleTexts {
		formattedExample = append(formattedExample, exampleNumber+e[9:]+"\n")
	}
	return strings.TrimSpace(strings.Join(formattedExample, ""))
}

func handleGlossaryQuery(input string) string {
	defineRequests.Add(1)
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
			return "<b>" + strings.Title(query) + "</b>: " + a
		}

		if query == "source" {
			query = "source of damage"
		}
		if query == "cda" {
			query = "characteristic-defining ability"
		}

		customScorer := func(s1, s2 string) int {
			return fuzzy.Ratio(s1, s2)
		}
		if bestGuess, err := fuzzy.ExtractOne(query, rulesKeys, customScorer); err != nil {
			log.Info("InExact rules match", "Error", err)
		} else {
			log.Debug("InExact rules match", "Guess", bestGuess)
			if bestGuess.Score > 60 {
				defineText = strings.Join(rules[bestGuess.Match], "\n")
			}
		}
		if defineText == "" {
			if bestGuess, err := fuzzy.ExtractOne(query, abilityWordKeys, customScorer); err != nil {
				log.Info("InExact aw match", "Error", err)
			} else {
				log.Debug("InExact aw match", "Guess", bestGuess)
				if bestGuess.Score > 60 {
					return "<b>" + strings.Title(bestGuess.Match) + "</b>: " + abilityWords[bestGuess.Match]
				}
			}
		}
	}
	// Some crappy workaround/s
	if !strings.HasPrefix(defineText, "<b>Dies</b>:") {
		defineText += tryFindSeeMoreRule(defineText)
	}
	return strings.TrimSpace(defineText)
}

func handleRulesQuery(input string) string {
	log.Debug("in handleRulesQuery (Volo)", "Input", input)

	// Hit examples first so it doesn't get consumed as a rule
	if (strings.HasPrefix(input, "ex") || strings.HasPrefix(input, "example")) && ruleRegexp.MatchString(input) {
		return handleExampleQuery(input)
	}

	if ruleRegexp.MatchString(input) {
		rulesRequests.Add(1)
		foundRuleNum := ruleRegexp.FindAllStringSubmatch(input, -1)[0][1]

		log.Debug("In handleRulesQuery (Volo)", "Rules matched on", foundRuleNum)
		if stringSliceContains(tooLongRules, foundRuleNum) {
			return fmt.Sprintf("<b>%s.</b> <i>[This subtype list is too long for chat. Please see %s ]</i>", foundRuleNum, voloSpecificRuleEndpointURL+foundRuleNum)
		}

		foundRule, err := findRule(foundRuleNum, "rule")
		if err != nil {
			return "Rule not found"
		}
		ruleText := foundRule.RuleText
		ruleNumber := []string{"<b>", foundRule.RuleNumber, ".</b> "}
		ruleWithNumber := append(ruleNumber, ruleText, "\n")
		return strings.TrimSpace(strings.Join(ruleWithNumber, ""))
	}

	// Glossary stuff in case someone's silly and did 'rule deathtouch'
	if strings.HasPrefix(input, "def ") || strings.HasPrefix(input, "define ") || strings.HasPrefix(input, "rule ") || strings.HasPrefix(input, "r ") || strings.HasPrefix(input, "cr ") {
		return handleGlossaryQuery(input)
	}
	// Somehow nothing matched?
	return ""
}
