package main

import (
	"context"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	blizzard "github.com/FuzzyStatic/blizzard/v2"
	"github.com/FuzzyStatic/blizzard/v2/wowgd"
	"github.com/algolia/algoliasearch-client-go/algolia/search"
	raven "github.com/getsentry/raven-go"
	lru "github.com/hashicorp/golang-lru"
	cache "github.com/patrickmn/go-cache"
	"github.com/slack-go/slack"
	hbot "github.com/whyrusleeping/hellabot"
	log "gopkg.in/inconshreveable/log15.v2"
)

// Configuration lists the configurable parameters, stored in config.json
type configuration struct {
	DSN          string   `json:"DSN"`
	Password     string   `json:"Password"`
	DevMode      bool     `json:"DevMode"`
	Server       struct {
		SSL 	string `json:SSL`
		NonSSL  string `json:NonSSL`
	}
	ProdChannels []string `json:"ProdChannels"`
	DevChannels  []string `json:"DevChannels"`
	Ops          []string `json:"Ops"`
	ProdNick     string   `json:"ProdNick"`
	DevNick      string   `json:"DevNick"`
	SlackTokens  []string `json:"SlackTokens"`
	Hearthstone  struct {
		AppID     string `json:"AppID"`
		APIToken  string `json:"APIToken"`
		IndexName string `json:"IndexName"`
	} `json:"Hearthstone"`
	BattleNet struct {
		ClientID         string   `json:"ClientID"`
		ClientSecret     string   `json:"ClientSecret"`
		CurrentExpansion string   `json:"CurrentExpansion"`
		CurrentRaidTier  string   `json:"CurrentRaidTier"`
		Reputations      []string `json:"Reputations"`
	} `json:"BattleNet"`
	PoE struct {
		League           string   `json:"League"`
		WantedCurrencies []string `json:"WantedCurrencies"`
	} `json:"PoE"`
	IRC   bool `json:"IRC"`
	Slack bool `json:"Slack"`
}

var (
	bot          *hbot.Bot
	conf         configuration
	ctx          context.Context
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
	rules           = make(map[string][]string)
	rulesKeys       []string
	abilityWords    = make(map[string]string)
	abilityWordKeys []string
	// Card names catalog
	cardNames      []string
	shortCardNames = make(map[string]string)

	highlanderPoints = make(map[string]int)

	// How often to dump the card cache
	cacheDumpTimer = 10 * time.Minute

	// Hearthstone clients
	hsClient *search.Client
	hsIndex  *search.Index

	// BattleNet client
	bNetClient           *blizzard.Client
	wowPlayerCache       *cache.Cache
	wowPlayerChieveCache *cache.Cache
	wowRealms            *wowgd.RealmIndex
	wowChieves           *wowgd.AchievementIndex
)

// Is there a stable URL that always points to a text version of the most up to date CR ?
// Fuck it Volo will do it
const crURL = "http://cr.vensersjournal.com"
const crFile = "CR.txt"
const cardCacheGob = "cardcache.gob"
const configFile = "config.json"
const abilityWordFile = "ability_words.json"
const cardShortNameFile = "short_names.json"

// CardGetter defines a function that retrieves a card's text.
// Defining this type allows us to override it in testing, and not hit scryfall.com a million times.
type CardGetter func(cardname string, isLang bool) (Card, error)

// RandomCardGetter defines a function that retrieves a random card's text.
type RandomCardGetter func() (Card, error)

// MultipleCardGetter defines a function that retrieves a bunch of cards.
type MultipleCardGetter func(searchTokens []string) ([]Card, error)

// fryatogParams contains the common things passed to and from functions.
type fryatogParams struct {
	m                     *hbot.Message
	slackm                string
	isIRC                 bool
	message               string
	fullInput             string
	cardGetFunction       CardGetter
	dumbCardGetFunction   CardGetter
	randomCardGetFunction RandomCardGetter
	cardFindFunction      MultipleCardGetter
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

func init() {
	expvar.Publish("Goroutines", expvar.Func(goRoutines))
}

func printHelp() string {
	var ret []string
	ret = append(ret, "!cardname to bring up that card's rules text")
	ret = append(ret, "!reminder <cardname> to bring up that card's reminder text")
	ret = append(ret, "!ruling <cardname> [ruling number] to bring up Gatherer rulings")
	ret = append(ret, "!rule <rulename> to bring up a Comprehensive Rule entry")
	ret = append(ret, "!define <glossary> to bring up the definition of a term")
	ret = append(ret, "!uncard/vanguard/plane/scheme <cardname> to bring up normally filtered out cards")
	ret = append(ret, "!url <mtr/ipg/cr/jar> to bring up the links to policy documents")
	ret = append(ret, "!roll <X> to roll X-sided die; !roll <XdY> to roll X Y-sided dice")
	ret = append(ret, "!coin to flip a coin (heads/tails)")
	ret = append(ret, "https://github.com/Fryyyyy/Fryatog/issues for bugs & feature requests")
	return strings.Join(ret, " · ")
}

func isSenderAnOp(m *hbot.Message) bool {
	conf = readConfig()
	/*whoChan = make(chan []string)
	getWho()
	var whoMessages [][]string
	for op := range whoChan {
		whoMessages = append(whoMessages, op)
	}
	handleWhoMessages(whoMessages)
	_, ok := chanops[m.From]*/
	for _, name := range conf.Ops {
		if m.From == name {
			return true
		}
	}
	return false
}

// tokeniseAndDispatchInput splits the given user-supplied string into a number of commands
// and does some pre-processing to sort out real commands from just normal chat
// Any real commands are handed to the handleCommand function
func tokeniseAndDispatchInput(fp *fryatogParams, cardGetFunction CardGetter, dumbCardGetFunction CardGetter, randomCardGetFunction RandomCardGetter, cardFindFunction MultipleCardGetter) []string {
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

	// Only start processing from the first ! or [[, not from the first &
	if strings.Contains(input, "&") {
		firstCmdIdx := max(cappedMin(strings.Index(input, "!"), strings.Index(input, "[["), -1), -1)
		if strings.Index(input, "&") < firstCmdIdx {
			input = input[firstCmdIdx:]
		}
	}

	// Little bit of hackery for PMs
	if !strings.Contains(input, "!") && !strings.Contains(input, "[[") {
		input = "!" + input
	}

	if strings.Contains(input, "http") || strings.Contains(input, ".com") {
		log.Info("URL skip", "Message", input)
		return []string{}
	}

	// Undo HTML encoding of operators
	input = strings.Replace(input, "&gt;", ">", -1)
	input = strings.Replace(input, "&lt;", "<", -1)

	commandList := botCommandRegex.FindAllString(input, -1)
	// log.Debug("Beginning T.I", "CommandList", commandList)
	c := make(chan string)
	var commands int
	var err error

	if isIRC {
		// Special case the Operator Commands
		switch {
		case input == "!quitquitquit" && isSenderAnOp(fp.m):
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(syscall.SIGQUIT)
		case strings.HasPrefix(input, "!cachedelete") && isSenderAnOp(fp.m):
			if err := deleteItemFromCache(normaliseCardName(input[12:])); err != nil {
				return []string{err.Error()}
			}
			return []string{"Done!"}
		case input == "!updatecardnames" && isSenderAnOp(fp.m):
			cardNames, err = importCardNames(true)
			if err != nil {
				log.Warn("Error importing card names", "Error", err)
				return []string{"Problem!"}
			}
			return []string{"Done!"}
		case input == "!startup" && isSenderAnOp(fp.m):
			var ret []string
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
		totalQueries.Add(1)
		if isIRC {
			ircQueries.Add(1)
		} else {
			slackQueries.Add(1)
		}
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
		message = strings.TrimPrefix(message, "card ")

		// Longest possible card name query is ~30 chars

		// TODO: Right now this is very magic number-y.
		//
		// Last time it bit us, the query '!ruling kozilek the great distortion 1'
		// was getting chopped off because we had this capped at 35.
		// Maybe look for some way to make this more robust and Actually Programmatic.
		if !strings.HasPrefix(message, "search ") && !strings.HasPrefix(message, "wow") && len(message) > 41 {
			message = message[0:41]
		}

		log.Debug("Dispatching", "index", commands)
		params := fryatogParams{message: message, fullInput: input, isIRC: isIRC, cardGetFunction: cardGetFunction, dumbCardGetFunction: dumbCardGetFunction, randomCardGetFunction: randomCardGetFunction, cardFindFunction: cardFindFunction}
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
	cardTokens := strings.Fields(message)
	log.Debug("Done tokenising", "Tokens", cardTokens)

	switch {

	case message == "help":
		log.Debug("Asked for help", "Input", message)
		c <- printHelp()
		return

	case cardTokens[0] == "hs" && !params.isIRC:
		log.Debug("Slack-based Hearthstone Query", "Input", message)
		c <- handleHearthstoneQuery(cardTokens[1:])
		return

	case cardTokens[0] == "wowchieve" && !params.isIRC:
		log.Debug("Slack-based Wow Chievo", "Input", message)
		switch len(cardTokens) {
		// Just bare command
		case 1:
			c <- "!wowchieve [realm] [player] <achievement name>"
		// Single Word Chieve Name
		case 2:
			c <- formatChieveForSlack(chieveFromID(chieveNameToID(cardTokens[1])))
		default:
			c <- handleChieveInput(message[10:])
		}
		return

	case cardTokens[0] == "wowstat" && !params.isIRC:
		log.Debug("Slack-based Wow Stat", "Input", message)
		switch len(cardTokens) {
		// Just bare command
		case 1:
			c <- "!wowstat [realm] [player] <statistic name or random>"
		default:
			c <- handleStatInput(message[8:])
		}
		return

	case cardTokens[0] == "wowstatfight" && !params.isIRC:
		log.Debug("Slack-based Wow Stat Fight", "Input", message)
		switch len(cardTokens) {
		// Just bare command
		case 1:
			c <- "!wowstatfight [realm1] [player1] [realm2] [player2] <statistic name or random>"
		default:
			c <- handleStatFightInput(message[13:])
		}
		return

	case cardTokens[0] == "wowdude" && !params.isIRC:
		log.Debug("Slack-based Wow Dude", "Input", message)
		switch len(cardTokens) {
		case 3:
			c <- printWoWDude(cardTokens[1], cardTokens[2])
		case 4:
			if cardTokens[1] == "raid" {
				c <- getDudeRaid(cardTokens[2], cardTokens[3], conf.BattleNet.CurrentExpansion, conf.BattleNet.CurrentRaidTier)
			} else if cardTokens[1] == "rep" {
				c <- getDudeReps(cardTokens[2], cardTokens[3])
			} else {
				c <- ""
			}
		default:
			c <- "!wowdude [raid/rep] <realm> <player>"
		}
		return

	case cardTokens[0] == "poecurrency" && !params.isIRC:
		log.Debug("Slack based PoE Currency", "Input", message)
		c <- handlePoeCurrencyQuery()
		return

	case cardTokens[0] == "icc" && (len(cardTokens) == 4 || len(cardTokens) == 2) && !params.isIRC:
		log.Debug("Slack-based ICC", "Input", message)
		var p []string
		if len(cardTokens) == 2 {
			p = strings.Split(cardTokens[1], "")
			if len(p) != 3 {
				c <- ""
				return
			}
		} else {
			p = cardTokens[1:]
		}
		c <- handleICC(p)
		return

	case cardTokens[0] == "mtr", cardTokens[0] == "ipg", cardTokens[0] == "url":
		log.Debug("Policy Query")
		c <- handlePolicyQuery(cardTokens)
		return

	case diceRegex.MatchString(message):
		log.Debug("Dice roll")
		c <- rollDice(message)
		return

	case message == "coin":
		log.Debug("Coin flip")
		c <- flipCoin()
		return

	case ruleRegexp.MatchString(message),
		strings.HasPrefix(message, "def "),
		strings.HasPrefix(message, "define "):
		log.Debug("Rules query", "Input", message)
		r := handleRulesQuery(message)
		if params.isIRC {
			c <- formatMessageForIRC(r)
		} else {
			c <- formatMessageForSlack(r)
		}
		return

	case cardMetadataRegex.MatchString(message):
		log.Debug("Metadata query")
		c <- handleCardMetadataQuery(params, cardTokens[0])
		return

	case cardTokens[0] == "search":
		log.Debug("Advanced search query", "Message", message, "Input", params.fullInput)
		// Before we search, make sure it's not the actual name of a card
		for _, x := range cardNames {
			if normaliseCardName(x) == normaliseCardName(message) {
				if card, err := findCard(cardTokens, false, params.cardGetFunction); err == nil {
					if params.isIRC {
						c <- card.formatCardForIRC()
					} else {
						c <- card.formatCardForSlack()
					}
					return
				}
			}
		}
		// If the search is the one and only thing
		if strings.HasPrefix(params.fullInput, "!search") && strings.Count(params.fullInput, "!") == 1 {
			log.Debug("Setting Full Input")
			cardTokens = strings.Fields(params.fullInput)
		}
		c <- strings.Join(handleAdvancedSearchQuery(params, cardTokens[1:]), "\n")
		return

	case cardTokens[0] == "uncard", cardTokens[0] == "vanguard", cardTokens[0] == "plane", cardTokens[0] == "scheme":
		log.Debug("Special card query", "Input", message)
		if card, err := findCard(cardTokens[1:], false, params.dumbCardGetFunction); err == nil {
			if params.isIRC {
				c <- card.formatCardForIRC()
			} else {
				c <- card.formatCardForSlack()
			}
			return
		}

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

	case cardTokens[0] == "en", cardTokens[0] == "es", cardTokens[0] == "fr", cardTokens[0] == "de", cardTokens[0] == "it" && cardTokens[1] != "that", cardTokens[0] == "pt", cardTokens[0] == "ja", cardTokens[0] == "ko", cardTokens[0] == "ru", cardTokens[0] == "zhs", cardTokens[0] == "zht":
		log.Debug("Asked for card in language", "Input", message)
		// Before we search for the language, make sure it's not the actual name of a card
		var found bool
		for _, x := range cardNames {
			if normaliseCardName(x) == normaliseCardName(message) {
				found = true
			}
		}
		if !found {
			isLang := true
			if cardTokens[0] == "en" {
				isLang = false
			}
			if card, err := findCard(cardTokens[1:], isLang, params.cardGetFunction); err == nil {
				translatedCard, err := card.cardGetLang(cardTokens[0])
				if err == nil {
					if params.isIRC {
						c <- translatedCard.formatCardForIRC()
					} else {
						c <- translatedCard.formatCardForSlack()
					}
					return
				}
			}
		}

	case cardTokens[0] == "wc" && params.isIRC:
		log.Debug("Asked for redirecting a user to rules")
		c <- sendRulesRedirectText(cardTokens)
		return

	default:
		log.Debug("I think it's a card")
		if card, err := findCard(cardTokens, false, params.cardGetFunction); err == nil {
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
}

func sendRulesRedirectText(cardTokens []string) string {
	if len(cardTokens) == 1 {
		return "Rules questions belong in the rules channel, not in here. Click #magicjudges-rules or type '/join #magicjudges-rules' (without the quotes) to get there"
	}
	return fmt.Sprintf("%s: Rules questions belong in the rules channel, not in here. Click #magicjudges-rules or type '/join #magicjudges-rules' (without the quotes) to get there", cardTokens[1])
}

func handleAdvancedSearchQuery(params *fryatogParams, cardTokens []string) []string {
	var ret []string
	cs, err := params.cardFindFunction(cardTokens)
	if err != nil {
		return []string{err.Error()}
	}
	for _, c := range cs {
		if params.isIRC {
			ret = append(ret, c.formatCardForIRC())
		} else {
			ret = append(ret, c.formatCardForSlack())
		}
	}
	return ret
}

func handleCardMetadataQuery(params *fryatogParams, command string) string {
	var (
		err          error
		rulingNumber int
	)
	command = strings.ToLower(command)
	if command == "reminder" {
		c, err := findCard(strings.Fields(params.message)[1:], false, params.cardGetFunction)
		if err != nil {
			return "Card not found"
		}
		return c.getReminderTexts()
	}
	if command == "flavor" || command == "flavour" {
		c, err := findCard(strings.Fields(params.message)[1:], false, params.cardGetFunction)
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
		if strings.HasPrefix(strings.ToLower(params.message), "ruling") {
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
		c, err := findCard(strings.Split(cardName, " "), false, params.cardGetFunction)
		if err != nil {
			return "Unable to find card"
		}
		return c.getRulings(rulingNumber)
	}

	log.Warn("handleCardMetadataQuery - didn't know what to do", "command", command, "input", params.message)
	return ""
}

func findCard(cardTokens []string, isLang bool, cardGetFunction CardGetter) (Card, error) {
	var bestCardSoFar Card
	for _, rc := range reduceCardSentence(cardTokens) {
		card, err := cardGetFunction(rc, isLang)
		log.Debug("Card Func gave us", "CardID", card.ID, "Err", err)
		if err == nil {
			log.Debug("Found card!", "Token", rc, "CardID", card.ID)
			if card.Lang == "en" && !isLang {
				return card, nil
			}
			// Initialise with the longest/best card
			if bestCardSoFar.ID == "" {
				bestCardSoFar = card
			}
		}
	}
	if bestCardSoFar.ID != "" {
		return bestCardSoFar, nil
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

func main() {
	flag.Parse()
	conf = readConfig()
	raven.SetDSN(conf.DSN)

	var err error
	// Bail out of everything if we can't have the rules.
	if err = importRules(false); err != nil {
		log.Warn("Error importing the rules", "Err", err)
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	cardNames, err = importCardNames(true)
	if err != nil {
		log.Warn("Error fetching card names", "Err", err)
		raven.CaptureErrorAndWait(err, nil)
	}

	// Initialise Cardname cache
	nameToCardCache, err = lru.NewARC(50)
	if err != nil {
		log.Warn("Error initialising the ARC", "Err", err)
		raven.CaptureErrorAndWait(err, nil)
		panic(err)
	}

	// Initialise HL points
	err = importHighlanderPoints(true)
	if err != nil {
		log.Warn("Error importing Highlander points", "Err", err)
		raven.CaptureErrorAndWait(err, nil)
	}

	// Initialise Ability Words
	err = importAbilityWords()
	if err != nil {
		log.Warn("Error importing ability words", "Err", err)
		raven.CaptureErrorAndWait(err, nil)
	}

	// Initialise Short Names
	if importShortCardNames() != nil {
		log.Warn("Error importing short card names", "Err", err)
		raven.CaptureErrorAndWait(err, nil)
	}

	ctx = context.Background()

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

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
		if err != nil {
			raven.CaptureErrorAndWait(err, nil)
			panic(err)
		}
		whichChans = conf.DevChannels
		whichNick = conf.DevNick
		nonSSLServ := flag.String("server", conf.Server.NonSSL, "hostname and port for irc server to connect to")
		nick := flag.String("nick", conf.DevNick, "nickname for the bot")
		bot, err = hbot.NewBot(*nonSSLServ, *nick, hijackSession, devChannels, noSSLOptions, timeOut)

		for _, sc := range conf.SlackTokens {
			slackClients = append(slackClients, slack.New(sc, slack.OptionDebug(true)))
		}
	} else {
		whichChans = conf.ProdChannels
		whichNick = conf.ProdNick
		sslServ := flag.String("server", conf.Server.SSL, "hostname and port for irc server to connect to")
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

	// Initialise Hearthstone card search engine
	hsClient = search.NewClient(conf.Hearthstone.AppID, conf.Hearthstone.APIToken)
	hsIndex = hsClient.InitIndex(conf.Hearthstone.IndexName)

	// Initialise Battlenet client
	if conf.BattleNet.ClientSecret != "" {
		bNetClient = blizzard.NewClient(conf.BattleNet.ClientID, conf.BattleNet.ClientSecret, blizzard.US, blizzard.EnUS)

		err = bNetClient.AccessTokenRequest(ctx)
		if err != nil {
			bNetClient = nil
			log.Warn("Error authenticating to Blizzard services", "Err", err)
			raven.CaptureErrorAndWait(err, nil)
		}
		wowPlayerCache = cache.New(24*time.Hour, 1*time.Hour)
		wowPlayerChieveCache = cache.New(24*time.Hour, 1*time.Hour)
		wowRealms, _, err = bNetClient.WoWRealmIndex(ctx)
		if err != nil {
			log.Warn("Error retrieving Realms", "Err", err)
			raven.CaptureErrorAndWait(err, nil)
		}
		wowChieves, _, err = bNetClient.WoWAchievementIndex(ctx)
		if err != nil {
			log.Warn("Error retrieving Chieves", "Err", err)
			raven.CaptureErrorAndWait(err, nil)
		}
	}

	bot.AddTrigger(mainTrigger)
	bot.AddTrigger(whoTrigger)
	bot.AddTrigger(endOfWhoTrigger)
	bot.AddTrigger(greetingTrigger)
	bot.AddTrigger(joinTrigger)
	bot.Logger.SetHandler(log.StdoutHandler)

	go dumpCardCacheTimer(&conf, nameToCardCache)

	// Start metrics server
	go http.ListenAndServe(":8888", nil)

	exitChan := getExitChannel()
	go func() {
		<-exitChan
		dumpCardCache(&conf, nameToCardCache)
		// close(bot.Incoming) // This has a tendency to panic when messages are received on a closed channel
		os.Exit(0) // Exit cleanly so we don't get autorestarted by supervisord. Also note https://github.com/golang/go/issues/24284
	}()

	// Start Slack stuff
	if conf.Slack {
		for _, scs := range slackClients {
			rtm := scs.NewRTM()
			go rtm.ManageConnection()
			go runSlack(rtm, scs)
		}
	}

	// Start up bot (this blocks until we disconnect)
	if conf.IRC {
		bot.Run()
	} else {
		// Gotta main loop
		for {
			time.Sleep(1 * time.Minute)
		}
	}
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
		return m.Command == "PRIVMSG" && !(greetingRegexp.MatchString(m.Content)) && ((!strings.Contains(m.To, "#") && !strings.Contains(m.Trailing(), "VERSION")) || (strings.Contains(m.Content, "!") || strings.Contains(m.Content, "[[")))
	},
	Action: func(irc *hbot.Bot, m *hbot.Message) bool {
		defer recovery()
		totalLines.Add(1)
		ircLines.Add(1)
		log.Debug("Dispatching message", "From", m.From, "To", m.To, "Content", m.Content)
		if m.From == whichNick {
			log.Debug("Ignoring message from myself", "Input", m.Content)
		}
		toPrint := tokeniseAndDispatchInput(&fryatogParams{m: m}, getScryfallCard, getDumbScryfallCard, getRandomScryfallCard, searchScryfallCard)
		for _, s := range sliceUniqMap(toPrint) {
			var prefix string
			isPublic := strings.Contains(m.To, "#")
			// If it's not a PM, address them.
			if isPublic && !strings.Contains(s, "#magicjudges-rules") {
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
							if strings.Contains(sss, "<i>") && !strings.Contains(sss, "</i>") {
								sss = sss + "</i>"
							}
							if strings.Contains(sss, "</i>") && !strings.Contains(sss, "<i>") {
								sss = "<i>" + sss
							}
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
