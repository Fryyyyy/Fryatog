package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	raven "github.com/getsentry/raven-go"
	lru "github.com/hashicorp/golang-lru"
	"github.com/nlopes/slack"
	cache "github.com/patrickmn/go-cache"
	fuzzy "github.com/paul-mannino/go-fuzzywuzzy"
	hbot "github.com/whyrusleeping/hellabot"
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
	SlackTokens  []string `json:"SlackTokens"`
}

var (
	bot          *hbot.Bot
	conf         configuration
	slackClients []*slack.Client

	// Caches
	nameToCardCache      *lru.ARCCache
	recentCacheMap       = make(map[string]*cache.Cache)
	recentPeopleCacheMap = make(map[string]*cache.Cache)

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

	// How often to dump the card cache
	cacheDumpTimer = 10 * time.Minute
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

// fryatogParams contains the common things passed to and from functions.
type fryatogParams struct {
	m                     *hbot.Message
	slackm                string
	isIRC                 bool
	message               string
	cardGetFunction       CardGetter
	randomCardGetFunction RandomCardGetter
}

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

// tokeniseAndDispatchInput splits the given user-supplied string into a number of commands
// and does some pre-processing to sort out real commands from just normal chat
// Any real commands are handed to the handleCommand function
func tokeniseAndDispatchInput(fp *fryatogParams, cardGetFunction CardGetter, randomCardGetFunction RandomCardGetter) []string {
	var input string
	isIRC := (fp.m != nil)
	if isIRC {
		input = fp.m.Content
	} else if fp.slackm != "" {
		input = fp.slackm
	} else {
		log.Warn("A Global Message with neither IRC nor SLack")
		return []string{}
	}

	// Little bit of hackery for PMs
	if !strings.Contains(input, "!") && !strings.Contains(input, "[[") {
		input = "!" + input
	}

	commandList := botCommandRegex.FindAllString(input, -1)
	// log.Debug("Beginning T.I", "CommandList", commandList)
	c := make(chan string)
	var commands int

	if isIRC {
		// Special case the Operator Commands
		switch {
		case input == "!quitquitquit" && isSenderAnOp(fp.m):
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(syscall.SIGQUIT)
		case input == "!updaterules" && isSenderAnOp(fp.m):
			if err := importRules(true); err != nil {
				log.Warn("Error importing Rules", "Error", err)
				return []string{"Problem!"}
			}
			return []string{"Done!"}
		case input == "!updatecardnames" && isSenderAnOp(fp.m):
			var err error
			cardNames, err = importCardNames(true)
			if err != nil {
				log.Warn("Error importing card names", "Error", err)
				return []string{"Problem!"}
			}
			return []string{"Done!"}
		case input == "!startup" && isSenderAnOp(fp.m):
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
		case input == "!dumpcardcache" && isSenderAnOp(fp.m):
			if err := dumpCardCache(&conf, nameToCardCache); err != nil {
				raven.CaptureErrorAndWait(err, nil)
			}
			return []string{"Done!"}
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

		// TODO: Right now this is very magic number-y.
		//
		// Last time it bit us, the query '!ruling kozilek the great distortion 1'
		// was getting chopped off because we had this capped at 35.
		// Maybe look for some way to make this more robust and Actually Programmatic.
		if len(message) > 41 {
			message = message[0:41]
		}

		log.Debug("Dispatching", "index", commands)
		params := fryatogParams{message: message, isIRC: isIRC, cardGetFunction: cardGetFunction, randomCardGetFunction: randomCardGetFunction}
		go handleCommand(&params, c)
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
func handleCommand(params *fryatogParams, c chan string) {
	message := params.message
	log.Debug("In handleCommand", "Message", message)
	tokens := strings.Fields(message)
	log.Debug("Done tokenising", "Tokens", tokens)

	switch {

	case message == "help":
		log.Debug("Asked for help", "Input", message)
		c <- printHelp()
		return

	case tokens[0] == "mtr", tokens[0] == "ipg":
		log.Debug("Asked for a policy link")
		policyResult := HandlePolicyQuery(tokens)
		c <- policyResult
		return

	case ruleRegexp.MatchString(message),
		strings.HasPrefix(message, "def "),
		strings.HasPrefix(message, "define "):
		log.Debug("Rules query", "Input", message)
		c <- handleRulesQuery(message)
		return

	case cardMetadataRegex.MatchString(message):
		log.Debug("Metadata query")
		c <- handleCardMetadataQuery(params, tokens[0])
		return

	case message == "random":
		log.Debug("Asked for random card")
		if card, err := getRandomCard(params.randomCardGetFunction); err == nil {
			if params.isIRC {
				c <- card.formatCardForIRC()
			} else {
				c <- card.formatCardForSlack()
			}
			return
		}

	default:
		log.Debug("I think it's a card")
		if card, err := findCard(tokens, params.cardGetFunction); err == nil {
			log.Debug("Got ye card", "IRC?", params.isIRC)
			if params.isIRC {
				c <- card.formatCardForIRC()
			} else {
				c <- card.formatCardForSlack()
			}
			return
		}
	}
	// If we got here, no cards found.
	c <- ""
	return
}

func handleCardMetadataQuery(params *fryatogParams, command string) string {
	var (
		err          error
		rulingNumber int
	)
	if command == "reminder" {
		c, err := findCard(strings.Fields(params.message)[1:], params.cardGetFunction)
		if err != nil {
			return "Card not found"
		}
		return c.getReminderTexts()
	}
	if command == "flavor" || command == "flavour" {
		c, err := findCard(strings.Fields(params.message)[1:], params.cardGetFunction)
		if err != nil {
			return "Card not found"
		}
		return c.getFlavourText()
	}

	if gathererRulingRegex.MatchString(strings.SplitN(params.message, " ", 2)[1]) {
		var cardName string
		fass := gathererRulingRegex.FindAllStringSubmatch(strings.SplitN(params.message, " ", 2)[1], -1)
		// One of these is guaranteed to contain the name
		cardName = fass[0][2] + fass[0][3] + fass[0][5]
		if len(cardName) == 0 {
			log.Debug("In a Ruling Query", "Couldn't find card name", params.message)
			return ""
		}
		if strings.HasPrefix(params.message, "ruling") {
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
		c, err := findCard(strings.Split(cardName, " "), params.cardGetFunction)
		if err != nil {
			return "Unable to find card"
		}
		return c.getRulings(rulingNumber)
	}

	log.Warn("handleCardMetadataQuery - didn't know what to do", "command", command, "input", params.message)
	return ""
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

func findCard(tokens []string, cardGetFunction CardGetter) (Card, error) {
	for _, rc := range reduceCardSentence(tokens) {
		card, err := cardGetFunction(rc)
		log.Debug("Card Func gave us", "CardID", card.ID, "Err", err)
		if err == nil {
			log.Debug("Found card!", "Token", rc, "CardID", card.ID)
			return card, nil
		}
	}
	return Card{}, fmt.Errorf("Card not found")
}

func getRandomCard(randomCardGetFunction RandomCardGetter) (Card, error) {
	card, err := randomCardGetFunction()
	if err == nil {
		log.Debug("Found card!", "CardID", card.ID)
		return card, nil
	}
	return Card{}, fmt.Errorf("Error retrieving random card")
}

func reduceCardSentence(tokens []string) []string {
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
		log.Warn("Error importing the rules", "Err", err)
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	cardNames, err = importCardNames(false)
	if err != nil {
		log.Warn("Error fetching card names", "Err", err)
		raven.CaptureErrorAndWait(err, nil)
	}

	// Initialise Cardname cache
	nameToCardCache, err = lru.NewARC(2048)
	if err != nil {
		log.Warn("Error initialising the ARC", "Err", err)
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
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
		// Make cache small in Debug mode, just for Volo
		nameToCardCache, err = lru.NewARC(2)
		whichChans = conf.DevChannels
		whichNick = conf.DevNick
		nonSSLServ := flag.String("server", "irc.freenode.net:6667", "hostname and port for irc server to connect to")
		nick := flag.String("nick", conf.DevNick, "nickname for the bot")
		bot, err = hbot.NewBot(*nonSSLServ, *nick, hijackSession, devChannels, noSSLOptions, timeOut)

		for _, sc := range conf.SlackTokens {
			slackClients = append(slackClients, slack.New(sc, slack.OptionDebug(true)))
		}
	} else {
		whichChans = conf.ProdChannels
		whichNick = conf.ProdNick
		sslServ := flag.String("server", "irc.freenode.net:6697", "hostname and port for irc server to connect to")
		nick := flag.String("nick", conf.ProdNick, "nickname for the bot")
		bot, err = hbot.NewBot(*sslServ, *nick, noHijackSession, prodChannels, yesSSLOptions, saslOptions, timeOut)

		for _, sc := range conf.SlackTokens {
			slackClients = append(slackClients, slack.New(sc))
		}

	}
	if err != nil {
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	// Load existing cards if necessary
	var cardsIn []Card
	err = readGob(cardCacheGob, &cardsIn)
	if err != nil {
		log.Warn("Error importing dumped card cache", "Err", err)
		raven.CaptureErrorAndWait(err, nil)
	}
	log.Debug("Found previously cached cards", "Number", len(cardsIn))
	for _, c := range cardsIn {
		// log.Debug("Adding card", "Name", c.Name)
		nameToCardCache.Add(normaliseCardName(c.Name), c)
	}

	// Initialise per-channel recent cache
	for _, channelName := range whichChans {
		log.Debug("Initialising cache", "Channel name", channelName)
		// Expires in 30 seconds, checks every 1 second
		recentCacheMap[channelName] = cache.New(30*time.Second, 1*time.Second)

		// And for greeting new-joiners
		if strings.Contains(channelName, "-rules") {
			log.Debug("Initialising new joiner cache", "Channel name", channelName)
			recentPeopleCacheMap[channelName] = cache.New(30*time.Second, 1*time.Second)
		}
	}

	bot.AddTrigger(mainTrigger)
	bot.AddTrigger(whoTrigger)
	bot.AddTrigger(endOfWhoTrigger)
	bot.AddTrigger(greetingTrigger)
	bot.AddTrigger(joinTrigger)
	bot.Logger.SetHandler(log.StdoutHandler)

	go dumpCardCacheTimer(&conf, nameToCardCache)

	exitChan := getExitChannel()
	go func() {
		<-exitChan
		dumpCardCache(&conf, nameToCardCache)
		// close(bot.Incoming) // This has a tendency to panic when messages are received on a closed channel
		os.Exit(0) // Exit cleanly so we don't get autorestarted by supervisord. Also note https://github.com/golang/go/issues/24284
	}()

	// Start Slack stuff
	for _, scs := range slackClients {
		rtm := scs.NewRTM()
		go rtm.ManageConnection()
		go runSlack(rtm, scs)
	}

	// Start up bot (this blocks until we disconnect)
	bot.Run()
	fmt.Println("Bot shutting down.")
}

// mainTrigger handles all command input.
// It could contain multiple comamnds, so for a message,
// we need to figure out how to handle it and if it does contain commands, handle them
// The message should probably start with a "!" or at least individual commands within it should.
// Also supports [[Cardname]]
// Most of this code stolen from Frytherer [https://github.com/Fryyyyy/Frytherer]
var mainTrigger = hbot.Trigger{
	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
		return m.Command == "PRIVMSG" && !(greetingRegexp.MatchString(m.Content)) && ((!strings.Contains(m.To, "#") && !strings.Contains(m.Trailing, "VERSION")) || (strings.Contains(m.Content, "!") || strings.Contains(m.Content, "[[")))
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		defer recovery()
		log.Debug("Dispatching message", "From", m.From, "To", m.To, "Content", m.Content)
		if m.From == whichNick {
			log.Debug("Ignoring message from myself", "Input", m.Content)
		}
		toPrint := tokeniseAndDispatchInput(&fryatogParams{m: m}, getScryfallCard, getRandomScryfallCard)
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

// whoTrigger handles the reply from the WHO comamnd we send to figure out who are the ChanOps.
var whoTrigger = hbot.Trigger{
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
		return (m.Command == "PRIVMSG") && (greetingRegexp.MatchString(m.Content)) && (strings.Contains(m.To, "-rules"))
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		log.Debug("Got a greeting!", "From", m.From, "To", m.To, "Content", m.Content)
		if _, found := recentPeopleCacheMap[m.To].Get(m.From); found {
			irc.Reply(m, fmt.Sprintf("%s: Hello! If you have a question about Magic rules, please go ahead and ask.", m.From))
		} else {
			log.Debug("But they've been here a while")
		}
		return false
	},
}

var joinTrigger = hbot.Trigger{
	Condition: func(bot *hbot.Bot, m *hbot.Message) bool {
		return (m.Command == "JOIN") && (strings.Contains(m.To, "-rules"))
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		log.Debug("JOIN Trigger in Rules", "From", m.From, "To", m.To)
		recentPeopleCacheMap[m.To].Set(m.From, true, cache.DefaultExpiration)
		return false
	},
}
