package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"strconv"
	raven "github.com/getsentry/raven-go"
	log "gopkg.in/inconshreveable/log15.v2"
)

const rulesEndpointURL = "https://api.academyruins.com/cr/"
const glossaryEndpointURL = "https://api.academyruins.com/cr/glossary/"
const examplesEndpointURL = "https://api.academyruins.com/cr/example/"
const specificRuleEndpointURL = "https://yawgatog.com/resources/magic-rules/#R"

var tooLongRules = []string{"205.3i", "205.3j", "205.3m", "205.3n"}

// Rule stores the result of Rules queries from API
type Rule struct {
	RuleNumber   string   `json:"ruleNumber"`
	RuleText     string   `json:"ruleText"`
	ExampleTexts []string `json:"examples"`
}

// Rule stores the result of Glossary queries from API
type GlossaryTerm struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

func tryFindSeeMoreRule(input string) string {
	log.Debug("TFSMR: This is input", "Input", input)
	if strings.Contains(input, "A keyword ability that lets a player attach an Equipment") {
		matches := seeRuleRegexp.FindAllStringSubmatch(input, -1)
		return "\n" + handleRulesQuery(matches[1][1]+"a")
	}

	if strings.Contains(input, "See rule") && !strings.Contains(input, "See rules") && !strings.Contains(input, "and rule") {
		matches := seeRuleRegexp.FindAllStringSubmatch(input, -1)
		if strings.Contains(input, "The object that dealt that damage") {
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
		endpoint = examplesEndpointURL
	case "rule":
		endpoint = rulesEndpointURL
	default:
		log.Error("findRule", "Called with unknown mode", which)
		return Rule{}, errors.New("Unknown mode")
	}

	url := endpoint + input + "?find_definition=true"
	log.Debug("findRule: Attempting to fetch", "URL", url)
	resp, err := http.Get(url)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Debug("HTTP request to Rules Endpoint failed", "Error", err)
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
	var (
		foundRuleNum string
		exampleIndex int
		err			 error
	)
	exampleRequests.Add(1)
	fss := ruleExampleRegexp.FindStringSubmatch(input)
	foundRuleNum = fss[2] + fss[3] + fss[5]
	if fss[1] == "" && fss[4] == "" {
		exampleIndex = 0
	} else {
		exampleIndex, err = strconv.Atoi(fss[1] + fss[4])
		if err != nil {
			return "Unable to parse example number"
		}
	}
	log.Debug("In handleExampleQuery", "Example matched on", foundRuleNum)

	foundExample, err := findRule(foundRuleNum, "example")

	if err != nil || foundExample.ExampleTexts == nil {
		return "Example not found"
	}

	if len(foundExample.ExampleTexts) < exampleIndex {
		return fmt.Sprintf("Too few examples for rule found (wanted %d, got %d)", exampleIndex, len(foundExample.ExampleTexts))
	}

	textsToPrint := foundExample.ExampleTexts
	if exampleIndex > 0 {
		textsToPrint = []string{textsToPrint[exampleIndex-1]}
	}

	var formattedExample []string
	exampleNumber := "<b>[" + foundExample.RuleNumber + "] Example:</b> "

	for _, e := range textsToPrint {
		formattedExample = append(formattedExample, exampleNumber+e+"\n")
	}
	return strings.TrimSpace(strings.Join(formattedExample, ""))
}

func handleGlossaryQuery(input string) string {
	defineRequests.Add(1)
	split := strings.SplitN(input, " ", 2)

	log.Debug("In handleGlossaryQuery", "Define matched on", split)
	query := TryCoerceGlossaryQuery(strings.ToLower(split[1]))

	url := glossaryEndpointURL + query + "?fuzzy=true"
	log.Debug("findGlossary: Attempting to fetch", "URL", url)
	resp, err := http.Get(url)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Debug("HTTP request to Glossary Endpoint failed", "Error", err)
		return ""
	}
	defer resp.Body.Close()
	var foundGlossaryTerm GlossaryTerm
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&foundGlossaryTerm); err != nil {
			raven.CaptureError(err, nil)
			log.Debug("Failed decoding the response", "Error", err)
			return ""
		}
	} else {
		return ""
	}

	// Some crappy workaround/s
	if foundGlossaryTerm.Term != "Dies" {
		foundGlossaryTerm.Definition += tryFindSeeMoreRule(foundGlossaryTerm.Definition)
	}
	return fmt.Sprintf("<b>%s</b>: %s", foundGlossaryTerm.Term, strings.TrimSpace(foundGlossaryTerm.Definition))
}

func TryCoerceGlossaryQuery(query string) string {
	if query == "cda" {
		query = "characteristic-defining ability"
	}
	if query == "source" {
		query = "source of damage"
	}
	return query
}

func handleRulesQuery(input string) string {
	log.Debug("in handleRulesQuery", "Input", input)

	// Hit examples first so it doesn't get consumed as a rule
	if (strings.HasPrefix(input, "ex") || strings.HasPrefix(input, "example")) && ruleRegexp.MatchString(input) {
		return handleExampleQuery(input)
	}

	if ruleRegexp.MatchString(input) {
		rulesRequests.Add(1)
		foundRuleNum := ruleRegexp.FindAllStringSubmatch(input, -1)[0][1]

		log.Debug("In handleRulesQuery", "Rules matched on", foundRuleNum)
		if stringSliceContains(tooLongRules, foundRuleNum) {
			foundRuleFragment := strings.Replace(foundRuleNum, ".", "", -1)
			return fmt.Sprintf("<b>%s.</b> <i>[This subtype list is too long for chat. Please see %s ]</i>", foundRuleNum, specificRuleEndpointURL+foundRuleFragment)
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
