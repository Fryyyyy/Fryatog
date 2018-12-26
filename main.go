package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	raven "github.com/getsentry/raven-go"
	lru "github.com/hashicorp/golang-lru"
	hbot "github.com/whyrusleeping/hellabot"
	charmap "golang.org/x/text/encoding/charmap"
	log "gopkg.in/inconshreveable/log15.v2"
)

var (
	nameToCardCache *lru.ARCCache

	// (for SSL) var serv = flag.String("server", "irc.freenode.net:6697", "hostname and port for irc server to connect to")
	serv = flag.String("server", "irc.freenode.net:6667", "hostname and port for irc server to connect to")
	nick = flag.String("nick", "Fryatog", "nickname for the bot")

	// Rules & Glossary dictionary.
	rules = make(map[string][]string)

	// Used in multiple functions.
	ruleRegexp = regexp.MustCompile(`((?:\d)+\.(?:\w{1,4}))`)
)

// Is there a stable URL that always points to a text version of the most up to date CR ?
// Fuck it I'll do it myself
const crURL = "https://chat.mtgpairings.info/cr-stable/"
const crFile = "CR.txt"

// CardGetter defines a function that retrieves a card's text.
// Defining this type allows us to override it in testing, and not hit scryfall.com a million times.
type CardGetter func(cardname string) (Card, error)

// TODO:
// Rules/CR
// Flavor/Flavour
// Help
// Operator commands to update CR
// Register bot nick
// Fuzzy matching on rules/defines
// LATER TODO:
// Advanced search
// Momir
// Support [[card]]
// Coin/D6
// Random

func fetchRulesFile() error {
	// Fetch it
	out, err := os.Create(crFile)
	if err != nil {
		return err
	}

	resp, err := http.Get(crURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	out.Close()
	return nil
}

func importRules() error {

	if _, err := os.Stat(crFile); err != nil {
		fetchRulesFile()
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
		metGlossary    bool
		metCredits     bool
		lastRule       string
		lastGlossary   string
		rulesMode      = true
		ruleParseRegex = regexp.MustCompile(`^(?P<ruleno>\d+\.\w{1,4})\.? (?P<ruletext>.*)`)
	)

	// Clear rules map
	rules = make(map[string][]string)

	// Begin rules parsing
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		// Clean up line
		line = strings.Replace(line, "“", `"`, -1)
		line = strings.Replace(line, "”", `"`, -1)
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
			if lastGlossary != "" {
				rules[lastGlossary] = append(rules[lastGlossary], line)
				lastGlossary = ""
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

// tokeniseAndDispatchInput splits the given user-supplied string into a number of commands
// and does some pre-processing to sort out real commands from just normal chat
// Any real commands are handed to the handleCommand function
func tokeniseAndDispatchInput(input string, cardGetFunction CardGetter) []string {
	var (
		botCommandRegex      = regexp.MustCompile(`[!&]([^!&]+)`)
		singleQuotedWord     = regexp.MustCompile(`^(?:\"|\')\w+(?:\"|\')$`)
		nonTextRegex         = regexp.MustCompile(`^[^\w]+$`)
		wordEndingInBang     = regexp.MustCompile(`\S+!(?: |\n)`)
		wordStartingWithBang = regexp.MustCompile(`\s+!(?: *)\S+`)
	)

	commandList := botCommandRegex.FindAllString(input, -1)
	// log.Debug("Beginning T.I", "CommandList", commandList)
	c := make(chan string)
	var commands int

	if wordEndingInBang.MatchString(input) && !wordStartingWithBang.MatchString(input) {
		log.Info("WEIB Skip")
		return []string{}
	}

	for _, message := range commandList {
		log.Debug("Processing:", "Command", message)
		if nonTextRegex.MatchString(message) || strings.HasPrefix(message, "  ") {
			log.Info("Iffy skip", "Message", message)
			continue
		}
		if strings.HasPrefix(message, "! ") {
			log.Info("Double Iffy Skip", "Message", message)
			continue
		}
		message = strings.TrimSpace(message)
		// Strip the command prefix
		if strings.HasPrefix(message, "!") || strings.HasPrefix(message, "&") {
			message = message[1:]
		}
		if singleQuotedWord.MatchString(message) {
			log.Debug("Single quoted word detected, stripping")
			message = message[1 : len(message)-1]
		}
		if message == "" {
			continue
		}
		if strings.HasPrefix(message, "card ") {
			message = message[5:]
		}

		// Longest possible card name query is ~30 chars
		if len(message) > 35 {
			message = message[0:35]
		}
		log.Debug("Dispatching", "index", commands)
		go handleCommand(message, c, cardGetFunction)
		commands++
	}
	var ret []string
	for i := 0; i < commands; i++ {
		log.Debug("Receiving", "index", i)
		ret = append(ret, <-c)
	}
	return ret
}

// handleCommand takes in a message, splits it into words
// and attempts to dispatch it to the correct handler.
func handleCommand(message string, c chan string, cardGetFunction CardGetter) {
	log.Debug("In HC", "Message", message)
	cardTokens := strings.Fields(message)
	log.Debug("Done tokenising", "Tokens", cardTokens)

	rulingOrFlavourRegex := regexp.MustCompile(`(?i)^((?:flavo(?:u{0,1})r(?: |s ))|(?:ruling(?: |s )))`)

	if ruleRegexp.MatchString(message) ||
		strings.HasPrefix(message, "r ") ||
		strings.HasPrefix(message, "cr ") ||
		strings.HasPrefix(message, "rule ") ||
		strings.HasPrefix(message, "def ") ||
		strings.HasPrefix(message, "define ") {
		log.Debug("Rules query", "Input", message)
		c <- handleRulesQuery(message)
		return
	}
	if rulingOrFlavourRegex.MatchString(message) {
		log.Debug("Ruling or flavour query")
		c <- handleRulingOrFlavourQuery(cardTokens[0], message, cardGetFunction)
		return
	}
	log.Debug("I think it's a card")
	if card, err := findCard(cardTokens, cardGetFunction); err == nil {
		c <- card.formatCard()
		return
	}
	// If we got here, no cards found.
	c <- ""
	return
}

func handleRulingOrFlavourQuery(command string, input string, cardGetFunction CardGetter) string {
	var (
		cardName            string
		rulingNumber        int
		gathererRulingRegex = regexp.MustCompile(`^(?:(?P<start_number>\d+) ?(?P<name>.+)|(?P<name2>.*?) ?(?P<end_number>\d+).*?|(?P<name3>.+))`)
	)
	if gathererRulingRegex.MatchString(strings.SplitN(input, " ", 2)[1]) {
		fass := gathererRulingRegex.FindAllStringSubmatch(strings.SplitN(input, " ", 2)[1], -1)
		// One of these is guaranteed to contain the name
		cardName = fass[0][2] + fass[0][3] + fass[0][5]
		if len(cardName) == 0 {
			log.Debug("In HROFQ", "Couldn't find card name", input)
			return ""
		}
		if strings.HasPrefix(input, "ruling") {
			rulingNumber, err := strconv.Atoi(fass[0][1] + fass[0][4])
			if err != nil {
				return "Unable to parse ruling number"
			}
			rulingNumber--
		}
		log.Debug("In HROFQ", "Valid command detected", "Command", command, "Card Name", cardName, "Ruling No.", rulingNumber)
		c, err := findCard(strings.Split(cardName, " "), cardGetFunction)
		if err != nil {
			return "Unable to find card"
		}
		return c.getRulings(rulingNumber)
	}
	return "RULING/FLAVOUR"
}

func handleRulesQuery(input string) string {
	log.Debug("In HRQ", "Input", input)
	// Match example first, for !ex101.a and !example 101.1a so the rule regexp doesn't eat it as a normal rule
	if (strings.HasPrefix(input, "ex") || strings.HasPrefix(input, "example ")) && ruleRegexp.MatchString(input) {
		log.Debug("In HRQ", "Example matched on", ruleRegexp.FindAllStringSubmatch(input, -1)[0][1])
		return strings.Join(rules["ex"+ruleRegexp.FindAllStringSubmatch(input, -1)[0][1]], "\n")
	}
	// Then try normal rules
	if ruleRegexp.MatchString(input) {
		log.Debug("In HRQ", "Rules matched on", ruleRegexp.FindAllStringSubmatch(input, -1)[0][1])
		return strings.Join(rules[ruleRegexp.FindAllStringSubmatch(input, -1)[0][1]], "\n")
	}
	// Finally try Glossary entries, people might do "!rule Deathtouch" rather than the proper "!define Deathtouch"
	if strings.HasPrefix(input, "def ") || strings.HasPrefix(input, "define ") || strings.HasPrefix(input, "rule ") || strings.HasPrefix(input, "r ") || strings.HasPrefix(input, "cr ") {
		log.Debug("In HRQ", "Define matched on", strings.SplitN(input, " ", 2))
		return strings.Join(rules[strings.SplitN(input, " ", 2)[1]], "\n")
	}
	// Didn't match ??
	return ""
}

func findCard(cardTokens []string, cardGetFunction CardGetter) (Card, error) {
	for _, rc := range reduceCardSentence(cardTokens) {
		card, err := cardGetFunction(rc)
		log.Debug("Card Func gave us", "CardID", card.ID, "Card", card, "Err", err)
		if err == nil {
			log.Debug("Found card!", "Token", rc, "CardID", card.ID, "Object", card)
			return card, nil
		}
	}
	return Card{}, fmt.Errorf("Card not found")
}

func reduceCardSentence(tokens []string) []string {
	noPunctuationRegex := regexp.MustCompile(`\W$`)
	log.Debug("In ReduceCard -- Tokens were", "Tokens", tokens, "Length", len(tokens))
	var ret []string
	for i := len(tokens); i >= 1; i-- {
		msg := strings.Join(tokens[0:i], " ")
		msg = noPunctuationRegex.ReplaceAllString(msg, "")
		log.Debug("Reverse descent", "i", i, "msg", msg)
		ret = append(ret, msg)
	}
	return ret
}

func main() {
	flag.Parse()
	raven.SetDSN("___DSN___")

	// Bail out of everything if we can't have the rules.
	if err := importRules(); err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	// Initialise cache
	var err error
	nameToCardCache, err = lru.NewARC(2048)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	hijackSession := func(bot *hbot.Bot) {
		bot.HijackSession = true
	}
	channels := func(bot *hbot.Bot) {
		bot.Channels = []string{"#frybottest"}
	}
	sslOptions := func(bot *hbot.Bot) {
		bot.SSL = false
	}
	irc, err := hbot.NewBot(*serv, *nick, hijackSession, channels, sslOptions)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	irc.AddTrigger(MainTrigger)
	irc.Logger.SetHandler(log.StdoutHandler)

	// Start up bot (this blocks until we disconnect)
	irc.Run()
	fmt.Println("Bot shutting down.")
}

// MainTrigger handles all command input.
// It could contain multiple comamnds, so for a message,
// we need to figure out how to handle it and if it does contain commands, handle them
// The message should probably start with a "!" or at least individual commands within it should.
// Most of this code stolen from Frytherer [https://github.com/Fryyyyy/Frytherer]
var MainTrigger = hbot.Trigger{
	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
		return m.Command == "PRIVMSG" && strings.Contains(m.Content, "!")
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		log.Debug("Dispatching message", "From", m.From, "To", m.To, "Content", m.Content)
		toPrint := tokeniseAndDispatchInput(m.Content, getScryfallCard)
		for _, s := range toPrint {
			if s != "" {
				irc.Reply(m, s)
			}
		}
		return false
	},
}
