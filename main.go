package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	raven "github.com/getsentry/raven-go"
	cache "github.com/patrickmn/go-cache"
	hbot "github.com/whyrusleeping/hellabot"
	charmap "golang.org/x/text/encoding/charmap"
	log "gopkg.in/inconshreveable/log15.v2"
)

var (
	// (for SSL) var serv = flag.String("server", "irc.freenode.net:6697", "hostname and port for irc server to connect to")
	serv = flag.String("server", "irc.freenode.net:6667", "hostname and port for irc server to connect to")
	nick = flag.String("nick", "Fryatog", "nickname for the bot")

	// Cache card results for a day.
	c = cache.New(24*time.Hour, 10*time.Minute)

	// Rules & Glossary dictionary.
	rules = make(map[string][]string)

	// Used in multiple functions.
	ruleRegexp = regexp.MustCompile(`(?:c?r(?:ule)?)?(?: )?((?:\d)+\.(?:\w{1,4}))`)
)

// CardGetter defines a function that retrieves a card's text.
// Defining this type allows us to override it in testing, and not hit scryfall.com a million times.
type CardGetter func(cardname string) (string, error)

// TODO:
// Rules/CR
// Flavor/Flavour
// Rulings
// Change cache
// Advanced search
// Momir ?
// Support [[card]] ?

func fetchOrImportRules() error {
	// Is there a stable URL that always points to a text version of the most up to date CR ?
	// Fuck it I'll do it myself
	// TODO: Need to know when the CR updates.  Probably name it after the date.
	const (
		crURL  = "https://chat.mtgpairings.info/cr-stable/"
		crFile = "CR.txt"
	)
	if _, err := os.Stat("CR.txt"); err != nil {
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
	}

	// Now CR file pretty much guaranteed to exist.
	// Let's parse it.
	// TODO: Examples
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
	botCommandRegex := regexp.MustCompile(`[!&]([^!&]+)`)
	singleQuotedWord := regexp.MustCompile(`^(?:\"|\')\w+(?:\"|\')$`)
	nonTextRegex := regexp.MustCompile(`^[^\w]+$`)
	wordEndingInBang := regexp.MustCompile(`\S+!(?: |\n)`)
	wordStartingWithBang := regexp.MustCompile(`\s+!(?: *)\S+`)

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
		if strings.HasPrefix(message, "!") {
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

	// gathererRuling_regex := regexp.MustCompile(`^(?:(?P<start_number>\d+) ?(?P<name>.+)|(?P<name2>.*?) ?(?P<end_number>\d+).*?|(?P<name3>.+))`)
	rulingOrFlavourRegex := regexp.MustCompile(`(?i)^((?:flavo(?:u{0,1})r(?: |s ))|(?:ruling(?: |s )))`)

	if ruleRegexp.MatchString(message) || strings.HasPrefix(message, "def ") || strings.HasPrefix(message, "define ") ||
		strings.HasPrefix(message, "ex ") {
		log.Debug("Rules query", "Input", message)
		c <- handleRulesQuery(message)
		return
	}
	if rulingOrFlavourRegex.MatchString(message) {
		log.Debug("Ruling or flavour query")
		c <- "Ruling or Flavour query"
		return
	}
	log.Debug("I think it's a card")
	for _, rc := range reduceCardSentence(cardTokens) {
		card, err := cardGetFunction(rc)
		log.Debug("Card Func gave us", "Card", card, "Err", err)
		if err == nil {
			log.Debug("Found card!", "Token", rc, "Text", card)
			c <- card
			return
		}
	}
	// If we got here, no cards found.
	c <- ""
	return
}

func handleRulesQuery(input string) string {
	log.Debug("In HRQ", "Input", input)
	if strings.HasPrefix(input, "def ") || strings.HasPrefix(input, "define ") {
		log.Debug("In HRQ", "Define matched on", strings.SplitN(input, " ", 2))
		return strings.Join(rules[strings.SplitN(input, " ", 2)[1]], "\n")
	}
	if strings.HasPrefix(input, "ex ") || strings.HasPrefix(input, "example ") {
		log.Debug("In HRQ", "Example matched on", strings.SplitN(input, " ", 2))
		return strings.Join(rules["ex"+strings.SplitN(input, " ", 2)[1]], "\n")
	}
	if ruleRegexp.MatchString(input) {
		log.Debug("In HRQ", "Rules matched on", ruleRegexp.FindAllStringSubmatch(input, -1)[0][1])
		return strings.Join(rules[ruleRegexp.FindAllStringSubmatch(input, -1)[0][1]], "\n")
	}
	// Didn't match ??
	return "RULEZ ???"
}

func normaliseCardName(input string) string {
	nonAlphaRegex := regexp.MustCompile(`\W+`)
	ret := nonAlphaRegex.ReplaceAllString(strings.ToLower(input), "")
	log.Debug("Normalising", "Input", input, "Output", ret)
	return ret
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

func getScryfallCard(input string) (string, error) {
	// Normalise input to match how we store in the cache:
	// lowercase, no punctuation.
	ncn := normaliseCardName(input)
	if cacheCard, found := c.Get(ncn); found {
		log.Debug("Card was cached")
		return cacheCard.(string), nil
	}
	url := fmt.Sprintf("https://api.scryfall.com/cards/named?fuzzy=%s", url.QueryEscape(input))
	log.Debug("Attempting to fetch", "URL", url)
	resp, err := http.Get(url)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Warn("The HTTP request failed", "Error", err)
		return "", fmt.Errorf("Something went wrong fetching the card")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var card Card
		if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
			raven.CaptureError(err, nil)
			return "", fmt.Errorf("Something went wrong parsing the card")
		}
		cardText := formatCard(&card)
		// Store what they typed, and also the real card name.
		c.Set(ncn, cardText, cache.DefaultExpiration)
		c.Set(normaliseCardName(card.Name), cardText, cache.DefaultExpiration)
		return cardText, nil
	}
	log.Info("Scryfall returned a non-200", "Status Code", resp.StatusCode)
	// Store nil result anyway.
	c.Set(ncn, "", cache.DefaultExpiration)
	return "", fmt.Errorf("No card found")
}

func main() {
	flag.Parse()
	raven.SetDSN("___DSN___")

	if err := fetchOrImportRules(); err != nil {
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
