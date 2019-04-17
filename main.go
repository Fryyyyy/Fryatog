package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	raven "github.com/getsentry/raven-go"
	lru "github.com/hashicorp/golang-lru"
	cache "github.com/patrickmn/go-cache"
	fuzzy "github.com/paul-mannino/go-fuzzywuzzy"
	hbot "github.com/whyrusleeping/hellabot"
	charmap "golang.org/x/text/encoding/charmap"
	log "gopkg.in/inconshreveable/log15.v2"
)

// Configuration lists the configurable parameters, stored in config.json
type configuration struct {
	DSN          string   `json:"DSN"`
	Password     string   `json:"Password"`
	DevMode      bool     `json:"DevMode"`
	ProdChannels []string `json:"ProdChannels"`
	DevChannels  []string `json:"DevChannels"`
	ProdNick     string   `json:"ProdNick"`
	DevNick      string   `json:"DevNick"`
}

var (
	bot  *hbot.Bot
	conf configuration

	// Caches
	nameToCardCache *lru.ARCCache
	recentCacheMap  = make(map[string]*cache.Cache)

	// IRC Variables
	whichChans []string
	whichNick  string
	whoChan    chan []string

	// Rules & Glossary dictionary.
	rules     = make(map[string][]string)
	rulesKeys []string
	// Card names catalog
	cardNames []string

	// Store people who we know of as Ops
	chanops = make(map[string]struct{})

	// Used in multiple functions.
	ruleRegexp     = regexp.MustCompile(`((?:\d)+\.(?:\w{1,4}))`)
	greetingRegexp = regexp.MustCompile(`(?i)^h(ello|i)(\!|\.|\?)*$`)
)

// Is there a stable URL that always points to a text version of the most up to date CR ?
// Fuck it I'll do it myself
const crURL = "https://chat.mtgpairings.info/cr-stable/"
const crFile = "CR.txt"
const cardCacheGob = "cardcache.gob"
const configFile = "config.json"

// CardGetter defines a function that retrieves a card's text.
// Defining this type allows us to override it in testing, and not hit scryfall.com a million times.
type CardGetter func(cardname string) (Card, error)

// RandomCardGetter defines a function that retrieves a random card's text.
type RandomCardGetter func() (Card, error)

func recovery() {
	if r := recover(); r != nil {
		// Log
		switch x := r.(type) {
		case string:
			raven.CaptureError(errors.New(x), nil)
		case error:
			raven.CaptureError(x, nil)
		default:
			raven.CaptureError(errors.New("unknown panic"), nil)
		}
		// If desired, actually quit
		if r == "quitquitquit" {
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(syscall.SIGQUIT)
		}
		// Else recover
	}
}

func readConfig() configuration {
	file, _ := os.Open(configFile)
	defer file.Close()
	decoder := json.NewDecoder(file)
	conf := configuration{}
	err := decoder.Decode(&conf)
	if err != nil {
		panic(err)
	}
	return conf
}

func printHelp() string {
	var ret []string
	ret = append(ret, "!cardname to bring up that card's rules text")
	ret = append(ret, "!reminder <cardname> to bring up that card's reminder text")
	ret = append(ret, "!ruling <cardname> [ruling number] to bring up Gatherer rulings")
	ret = append(ret, "!rule <rulename> to bring up a Comprehensive Rule entry")
	ret = append(ret, "!define <glossary> to bring up the definition of a term")
	return strings.Join(ret, " · ")
}

func isSenderAnOp(m *hbot.Message) bool {
	log.Debug("In isSenderAnOp", "Chanops", chanops)
	whoChan = make(chan []string)
	getWho()
	var whoMessages [][]string
	for op := range whoChan {
		whoMessages = append(whoMessages, op)
	}
	handleWhoMessages(whoMessages)
	_, ok := chanops[m.From]
	return ok
}

func handleWhoMessages(inputs [][]string) {
	for _, input := range inputs {
		log.Debug("Handling Who Middle", "len7", len(input) == 7, "whichChans", whichChans)
		// Input:
		// 0 Bot Nickname
		// 1 Channel
		// 2 User
		// 3 Host
		// 4 Server
		// 5 User Nick
		// 6 Modes
		if len(input) == 7 {
			log.Debug("Handling Who Middle", "hasAt", strings.Contains(input[6], "@"), "isInChan", stringSliceContains(whichChans, input[1]))
			// Are they an op in one of our Base channels?
			if strings.Contains(input[6], "@") && stringSliceContains(whichChans, input[1]) {
				chanops[input[5]] = struct{}{}
			}
		}
		log.Debug("Handling Who Message", "Chanops Result", chanops)
	}
}

func getWho() {
	log.Debug("Horton hears")
	// Clear existing chanops
	chanops = make(map[string]struct{})
	for _, c := range whichChans {
		bot.Send(fmt.Sprintf("WHO %s", c))
	}
}

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

// tokeniseAndDispatchInput splits the given user-supplied string into a number of commands
// and does some pre-processing to sort out real commands from just normal chat
// Any real commands are handed to the handleCommand function
func tokeniseAndDispatchInput(m *hbot.Message, cardGetFunction CardGetter, randomCardGetFunction RandomCardGetter) []string {
	var (
		botCommandRegex      = regexp.MustCompile(`[!&]([^!&?[)]+)|\[\[(.*?)\]\]`)
		singleQuotedWord     = regexp.MustCompile(`^(?:\"|\')\w+(?:\"|\')$`)
		nonTextRegex         = regexp.MustCompile(`^[^\w]+$`)
		wordEndingInBang     = regexp.MustCompile(`!(?:"|') |(?:\n)+`)
		wordStartingWithBang = regexp.MustCompile(`\s+!(?: *)\S+`)
		input                = m.Content
	)

	// Little bit of hackery for PMs
	if !strings.Contains(input, "!") && !strings.Contains(input, "[[") {
		input = "!" + input
	}

	commandList := botCommandRegex.FindAllString(input, -1)
	// log.Debug("Beginning T.I", "CommandList", commandList)
	c := make(chan string)
	var commands int

	// Special case the Operator Commands
	switch {
	case input == "!quitquitquit" && isSenderAnOp(m):
		panic("quitquitquit")
	case input == "!updaterules" && isSenderAnOp(m):
		if err := importRules(true); err != nil {
			log.Warn("Error importing Rules", "Error", err)
			return []string{"Problem!"}
		}
		return []string{"Done!"}
	case input == "!updatecardnames" && isSenderAnOp(m):
		var err error
		cardNames, err = importCardNames(true)
		if err != nil {
			log.Warn("Error importing card names", "Error", err)
			return []string{"Problem!"}
		}
		return []string{"Done!"}
	case input == "!startup" && isSenderAnOp(m):
		var ret []string
		var err error
		if err = importRules(false); err != nil {
			ret = append(ret, "Problem fetching rules")
		}
		cardNames, err = importCardNames(false)
		if err != nil {
			ret = append(ret, "Problem fetching card names")
		}
		ret = append(ret, "Done!")
		return ret
	case input == "!dumpcardcache" && isSenderAnOp(m):
		if err := dumpCardCache(); err != nil {
			raven.CaptureErrorAndWait(err, nil)
		}
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
		if wordEndingInBang.MatchString(message) && !wordStartingWithBang.MatchString(message) {
			log.Info("WEIB Skip")
			continue
		}
		message = strings.TrimSpace(message)
		// Strip the command prefix
		if strings.HasPrefix(message, "!") || strings.HasPrefix(message, "&") {
			message = message[1:]
		}
		if strings.HasPrefix(message, "[[") && strings.HasSuffix(message, "]]") {
			message = message[2 : len(message)-2]
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
		go handleCommand(message, c, cardGetFunction, randomCardGetFunction)
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
func handleCommand(message string, c chan string, cardGetFunction CardGetter, randomCardGetFunction RandomCardGetter) {
	log.Debug("In handleCommand", "Message", message)
	cardTokens := strings.Fields(message)
	log.Debug("Done tokenising", "Tokens", cardTokens)

	cardMetadataRegex := regexp.MustCompile(`(?i)^(?:ruling(?:s?)|reminder|flavo(?:u?)r)(?: )`)

	switch {

	case message == "help":
		log.Debug("Asked for help", "Input", message)
		c <- printHelp()
		return

	case ruleRegexp.MatchString(message),
		strings.HasPrefix(message, "def "),
		strings.HasPrefix(message, "define "):
		log.Debug("Rules query", "Input", message)
		c <- handleRulesQuery(message)
		return

	case cardMetadataRegex.MatchString(message):
		log.Debug("Metadata query")
		c <- handleCardMetadataQuery(cardTokens[0], message, cardGetFunction)
		return

	case message == "random":
		log.Debug("Asked for random card")
		if card, err := getRandomCard(randomCardGetFunction); err == nil {
			c <- card.formatCard()
			return
		}

	default:
		log.Debug("I think it's a card")
		if card, err := findCard(cardTokens, cardGetFunction); err == nil {
			c <- card.formatCard()
			return
		}
	}
	// If we got here, no cards found.
	c <- ""
	return
}

func handleCardMetadataQuery(command string, input string, cardGetFunction CardGetter) string {
	var (
		err                 error
		rulingNumber        int
		gathererRulingRegex = regexp.MustCompile(`^(?:(?P<start_number>\d+) ?(?P<name>.+)|(?P<name2>.*?) ?(?P<end_number>\d+).*?|(?P<name3>.+))`)
	)
	if command == "reminder" {
		c, err := findCard(strings.Fields(input)[1:], cardGetFunction)
		if err != nil {
			return "Card not found"
		}
		return c.getReminderTexts()
	}
	if command == "flavor" || command == "flavour" {
		c, err := findCard(strings.Fields(input)[1:], cardGetFunction)
		if err != nil {
			return "Card not found"
		}
		return c.getFlavourText()
	}
	if gathererRulingRegex.MatchString(strings.SplitN(input, " ", 2)[1]) {
		var cardName string
		fass := gathererRulingRegex.FindAllStringSubmatch(strings.SplitN(input, " ", 2)[1], -1)
		// One of these is guaranteed to contain the name
		cardName = fass[0][2] + fass[0][3] + fass[0][5]
		if len(cardName) == 0 {
			log.Debug("In a Ruling Query", "Couldn't find card name", input)
			return ""
		}
		if strings.HasPrefix(input, "ruling") {
			// If there is no number, set to 0.
			if fass[0][1] == "" && fass[0][4] == "" {
				rulingNumber = 0
			} else {
				rulingNumber, err = strconv.Atoi(fass[0][1] + fass[0][4])
				if err != nil {
					return "Unable to parse ruling number"
				}
			}
		}
		log.Debug("In a Ruling Query - Valid command detected", "Command", command, "Card Name", cardName, "Ruling No.", rulingNumber)
		c, err := findCard(strings.Split(cardName, " "), cardGetFunction)
		if err != nil {
			return "Unable to find card"
		}
		return c.getRulings(rulingNumber)
	}

	return "RULING/FLAVOUR"
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
		exampleNumber := []string{"\x02[", foundRuleNum, "] Example:\x0F "}
		exampleText := strings.Join(rules["ex"+foundRuleNum], "")[9:]
		formattedExample := append(exampleNumber, exampleText, "\n")
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
		foundKeywordAbilityRegexp := regexp.MustCompile(`701.\d+\b`)
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
		foundKeywordActionRegexp := regexp.MustCompile(`702.\d+\b`)
		if foundKeywordActionRegexp.MatchString(input) {
			ruleText, foundRuleNum = tryFindBetterAbilityRule(ruleText, foundRuleNum)
		}
		ruleNumber := []string{"\x02", foundRuleNum, ".\x0F "}
		ruleWithNumber := append(ruleNumber, ruleText, "\n")
		return strings.TrimSpace(strings.Join(ruleWithNumber, ""))
	}
	// Finally try Glossary entries, people might do "!rule Deathtouch" rather than the proper "!define Deathtouch"
	if strings.HasPrefix(input, "def ") || strings.HasPrefix(input, "define ") || strings.HasPrefix(input, "rule ") || strings.HasPrefix(input, "r ") || strings.HasPrefix(input, "cr ") {
		log.Debug("In handleRulesQuery", "Define matched on", strings.SplitN(input, " ", 2))
		query := strings.SplitN(input, " ", 2)[1]
		var defineText string
		v, ok := rules[query]
		if ok {
			log.Debug("Exact match")
			defineText = strings.Join(v, "\n")
		} else {
			// Special case, otherwise it matches "Planar Die" better
			if query == "die" {
				query = "dies"
			}
			if bestGuess, err := fuzzy.ExtractOne(query, rulesKeys); err != nil {
				log.Info("InExact match", "Error", err)
			} else {
				log.Debug("InExact match", "Guess", bestGuess)
				if bestGuess.Score > 80 {
					defineText = strings.Join(rules[bestGuess.Match], "\n")
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

func tryFindSeeMoreRule(input string) string {
	var seeRuleRegexp = regexp.MustCompile(`See rule (\d+\.{0,1}\d*)`)
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

func getRandomCard(randomCardGetFunction RandomCardGetter) (Card, error) {
	card, err := randomCardGetFunction()
	if err == nil {
		log.Debug("Found card!", "CardID", card.ID, "Object", card)
		return card, nil
	}
	return Card{}, fmt.Errorf("Error retrieving random card")
}

func reduceCardSentence(tokens []string) []string {
	noPunctuationRegex := regexp.MustCompile(`\W$`)
	log.Debug("In ReduceCard -- Tokens were", "Tokens", tokens, "Length", len(tokens))
	var ret []string
	for i := len(tokens); i >= 1; i-- {
		msg := strings.Join(tokens[0:i], " ")
		msg = noPunctuationRegex.ReplaceAllString(msg, "")
		// Eliminate short names which are not valid and would match too much
		if len(msg) > 2 {
			log.Debug("Reverse descent", "i", i, "msg", msg)
			ret = append(ret, msg)
		}
	}
	return ret
}

func main() {
	flag.Parse()
	conf := readConfig()
	raven.SetDSN(conf.DSN)

	var err error
	// Bail out of everything if we can't have the rules.
	if err = importRules(false); err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	cardNames, err = importCardNames(false)
	if err != nil {
		log.Warn("Error fetching card names", "Err", err)
	}

	// Initialise Cardname cache
	nameToCardCache, err = lru.NewARC(2048)
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}
	// Load existing cards if necessary
	var cardsIn []Card
	err = readGob(cardCacheGob, &cardsIn)
	if err != nil {
		log.Warn("Error importing dumped card cache", "Err", err)
	}
	log.Debug("Found previously cached cards", "Number", len(cardsIn))
	for _, c := range cardsIn {
		log.Debug("Adding card", "Name", c.Name)
		nameToCardCache.Add(c.Name, c)
		nameToCardCache.Add(normaliseCardName(c.Name), c)
	}

	hijackSession := func(bot *hbot.Bot) {
		bot.HijackSession = true
	}
	noHijackSession := func(bot *hbot.Bot) {
		bot.HijackSession = false
	}
	devChannels := func(bot *hbot.Bot) {
		bot.Channels = conf.DevChannels
	}
	prodChannels := func(bot *hbot.Bot) {
		bot.Channels = conf.ProdChannels
	}
	noSSLOptions := func(bot *hbot.Bot) {
		bot.SSL = false
	}
	yesSSLOptions := func(bot *hbot.Bot) {
		bot.SSL = true
	}
	saslOptions := func(bot *hbot.Bot) {
		bot.SASL = true
		bot.Password = conf.Password
	}
	timeOut := func(bot *hbot.Bot) {
		bot.ThrottleDelay = 300 * time.Millisecond
	}
	if conf.DevMode {
		log.Debug("DEBUG MODE")
		whichChans = conf.DevChannels
		whichNick = conf.DevNick
		nonSSLServ := flag.String("server", "irc.freenode.net:6667", "hostname and port for irc server to connect to")
		nick := flag.String("nick", conf.DevNick, "nickname for the bot")
		bot, err = hbot.NewBot(*nonSSLServ, *nick, hijackSession, devChannels, noSSLOptions, timeOut)
	} else {
		whichChans = conf.ProdChannels
		whichNick = conf.ProdNick
		sslServ := flag.String("server", "irc.freenode.net:6697", "hostname and port for irc server to connect to")
		nick := flag.String("nick", conf.ProdNick, "nickname for the bot")
		bot, err = hbot.NewBot(*sslServ, *nick, noHijackSession, prodChannels, yesSSLOptions, saslOptions, timeOut)
	}
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	// Initialise per-channel recent cache
	for _, channelName := range whichChans {
		log.Debug("Initialising cache", "Channel name", channelName)
		recentCacheMap[channelName] = cache.New(30*time.Second, 1*time.Second)
	}

	bot.AddTrigger(MainTrigger)
	bot.AddTrigger(WhoTrigger)
	bot.AddTrigger(endOfWhoTrigger)
	bot.AddTrigger(greetingTrigger)
	bot.Logger.SetHandler(log.StdoutHandler)

	exitChan := getExitChannel()
	go func() {
		<-exitChan
		dumpCardCache()
		// close(bot.Incoming) // This has a tendency to panic when messages are received on a closed channel
		os.Exit(0) // Exit cleanly so we don't get autorestarted by supervisord. Also note https://github.com/golang/go/issues/24284
	}()

	// Start up bot (this blocks until we disconnect)
	bot.Run()
	fmt.Println("Bot shutting down.")
}

// MainTrigger handles all command input.
// It could contain multiple comamnds, so for a message,
// we need to figure out how to handle it and if it does contain commands, handle them
// The message should probably start with a "!" or at least individual commands within it should.
// Also supports [[Cardname]]
// Most of this code stolen from Frytherer [https://github.com/Fryyyyy/Frytherer]
var MainTrigger = hbot.Trigger{
	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
		return m.Command == "PRIVMSG" && !(greetingRegexp.MatchString(m.Content)) && ((!strings.Contains(m.To, "#") && !strings.Contains(m.Trailing, "VERSION")) || (strings.Contains(m.Content, "!") || strings.Contains(m.Content, "[[")))
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		defer recovery()
		log.Debug("Dispatching message", "From", m.From, "To", m.To, "Content", m.Content)
		if m.From == whichNick {
			log.Debug("Ignoring message from myself", "Input", m.Content)
		}
		toPrint := tokeniseAndDispatchInput(m, getScryfallCard, getRandomScryfallCard)
		for _, s := range sliceUniqMap(toPrint) {
			var prefix string
			isPublic := strings.Contains(m.To, "#")
			// If it's not a PM, address them
			if isPublic {
				prefix = fmt.Sprintf("%s: ", m.From)
			}
			if s != "" {
				// Check if we've already sent it recently (only for public channels)
				if isPublic {
					if _, found := recentCacheMap[m.To].Get(s); found && !strings.Contains(s, "not found") {
						// Safety net for the odd case where the cached string is shorter than 23 chars.
						maxLen := min(len(s), 23)
						irc.Reply(m, fmt.Sprintf("%sDuplicate response withheld. (%s ...)", prefix, s[:maxLen]))
						continue
					}
					recentCacheMap[m.To].Set(s, true, cache.DefaultExpiration)
				}
				for _, ss := range strings.Split(s, "\n") {
					{
						if ss == "" {
							continue
						}
						for i, sss := range strings.Split(wordWrap(ss, (390-len(prefix))), "\n") {
							if i == 0 {
								irc.Reply(m, fmt.Sprintf("%s%s", prefix, sss))
							} else {
								irc.Reply(m, sss)
							}
						}
					}
				}
			}
		}
		return false
	},
}

// WhoTrigger handles the reply from the WHO comamnd we send to
// figure out who are the ChanOps.
var WhoTrigger = hbot.Trigger{
	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
		// 352 is RPL_WHOREPLY https://tools.ietf.org/html/rfc1459#section-6.2
		// 315 is RPL_ENDOFWHO
		return m.Command == "352"
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		log.Debug("Got a WHO message!", "From", m.From, "To", m.To, "Params", m.Params)
		whoChan <- m.Params
		return false
	},
}

var endOfWhoTrigger = hbot.Trigger{
	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
		// 315 is RPL_ENDOFWHO https://tools.ietf.org/html/rfc1459#section-6.2
		return m.Command == "315"
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		log.Debug("Got an END OF WHO message!", "From", m.From, "To", m.To, "Params", m.Params)
		close(whoChan)
		return false
	},
}

var greetingTrigger = hbot.Trigger{
	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
		return (m.Command == "PRIVMSG") && (greetingRegexp.MatchString(m.Content))
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		log.Debug("Got a greeting!", "From", m.From, "To", m.To, "Content", m.Content)
		irc.Reply(m, fmt.Sprintf("%s: Hello! If you have a question about Magic rules, please go ahead and ask.", m.From))
		return false
	},
}
