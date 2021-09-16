package main

import (
	"encoding/gob"
	"encoding/json"
	"expvar"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	lru "github.com/hashicorp/golang-lru"
	log "gopkg.in/inconshreveable/log15.v2"
)

var (
	//Pulling all regex here *should* make it all compile once and then be left alone

	//Stuff pared from main.go
	botCommandRegex      = regexp.MustCompile(`[!&]([^=!&?[)][^!&?[)]+)\.\s|[!&]([^=!&?[)][^!&?[)]+)|\[\[(.*?)\]\]`)
	singleQuotedWord     = regexp.MustCompile(`^(?:\"|\')\w+(?:\"|\')$`)
	nonTextRegex         = regexp.MustCompile(`^[^\w]+$`)
	wordEndingInBang     = regexp.MustCompile(`!(?:"|') |(?:\n)+`)
	wordStartingWithBang = regexp.MustCompile(`\s+!(?: *)\S+`)

	diceRegex = regexp.MustCompile(`(?:roll)?(?:\s*)(.*?)(?:d(\d+)([+-]\d+)?)`)

	cardMetadataRegex = regexp.MustCompile(`(?i)^(?:ruling(?:s?)|reminder|flavo(?:u?)r)(?: )`)

	gathererRulingRegex = regexp.MustCompile(`^(?:(?P<start_number>\d+) ?(?P<name>.+)|(?P<name2>.*?) ?(?P<end_number>\d+).*?|(?P<name3>.+))`)

	ruleParseRegex = regexp.MustCompile(`^(?P<ruleno>\d+\.\w{1,4})\.? (?P<ruletext>.*)`)

	seeRuleRegexp = regexp.MustCompile(`See rule (\d+\.{0,1}\d*\w?)`)

	noPunctuationRegex = regexp.MustCompile(`\W$`)

	// Used in multiple functions.
	ruleRegexp     = regexp.MustCompile(`((?:\d)+\.(?:\w{1,4}))`)
	greetingRegexp = regexp.MustCompile(`(?i)^h(ello|i)( *)(\!|\.|\?)*( *)$`)

	//Stuff pared from card.go
	reminderRegexp = regexp.MustCompile(`\((.*?)\)`)
	nonAlphaRegex  = regexp.MustCompile(`\W+`)
	emojiRegex     = regexp.MustCompile(`{\d+}|{[A-Z]}|{\d\/[A-Z]}|{[A-Z]\/[A-Z]}`)

	// Super rudimentary policy regex to parse e.g '4.8' into 4-8 for link generation
	policyRegex = regexp.MustCompile(`[^0-9]+`)

	// We PROBABLY want Uro, not Aurochs
	uroRegex = regexp.MustCompile(`^[Uu]ro,?$`)

	// Metrics
	totalLines             = expvar.NewInt("bot_totalLines")
	ircLines               = expvar.NewInt("bot_ircLines")
	slackLines             = expvar.NewInt("bot_slackLines")
	totalQueries           = expvar.NewInt("bot_totalQueries")
	ircQueries             = expvar.NewInt("bot_ircQueries")
	slackQueries           = expvar.NewInt("bot_slackQueries")
	cardsInCache           = expvar.NewInt("bot_cardsInCache")
	cardCacheQueries       = expvar.NewInt("bot_cardCacheQueries")
	cardCacheHits          = expvar.NewInt("bot_cardCacheHits")
	cardCacheHitPercentage = expvar.NewInt("bot_cardCacheHitPercentage")
	cardUniquePrefixHits   = expvar.NewInt("bot_cardUniquePrefixHits")
	searchRequests         = expvar.NewInt("bot_searchRequests")
	randomRequests         = expvar.NewInt("bot_randomRequests")
	cardRequests           = expvar.NewInt("bot_cardRequests")
	dumbCardRequests       = expvar.NewInt("bot_dumbCardRequests")
	metadataRequests       = expvar.NewInt("bot_metadataRequests")
	reminderRequests       = expvar.NewInt("bot_reminderRequests")
	flavourRequests        = expvar.NewInt("bot_flavourRequests")
	rulingRequests         = expvar.NewInt("bot_rulingRequests")
	exampleRequests        = expvar.NewInt("bot_exampleRequests")
	rulesRequests          = expvar.NewInt("bot_rulesRequests")
	defineRequests         = expvar.NewInt("bot_defineRequests")
	hearthstoneRequests    = expvar.NewInt("bot_hearthstoneRequests")
)

func sliceUniqMap(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	j := 0
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		s[j] = v
		j++
	}
	return s[:j]
}

func stringSliceContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func wordWrap(text string, lineWidth int) string {
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		return text
	}
	wrapped := words[0]
	spaceLeft := lineWidth - len(wrapped)
	for _, word := range words[1:] {
		if len(word)+1 > spaceLeft {
			wrapped += " [...]\n" + word
			spaceLeft = lineWidth - len(word)
		} else {
			wrapped += " " + word
			spaceLeft -= 1 + len(word)
		}
	}
	return wrapped
}

func writeGob(filePath string, object interface{}) error {
	file, err := os.Create(filePath)
	if err == nil {
		encoder := gob.NewEncoder(file)
		encoder.Encode(object)
	} else {
		log.Warn("Error creating GOB file", "Error", err)
	}
	file.Close()
	return err
}

func readGob(filePath string, object interface{}) error {
	file, err := os.Open(filePath)
	if err == nil {
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(object)
		// log.Debug("readGOB", "Object", object)
	}
	file.Close()
	return err
}

func dumpCardCache(conf *configuration, cache *lru.ARCCache) error {
	// Dump cache keys
	log.Debug("Dumping card cache", "len", cache.Len())
	cardsInCache.Set(int64(cache.Len()))
	var outCards []Card
	for _, k := range cache.Keys() {
		if v, ok := cache.Peek(k); ok {
			if v != nil {
				if conf.DevMode {
					log.Debug("Dumping card", "Key", k, "Name", (v.(Card)).Name)
				}
				outCards = append(outCards, v.(Card))
			}
		}
	}
	return writeGob(cardCacheGob, outCards)
}

func getExitChannel() chan os.Signal {
	c := make(chan os.Signal, 1)
	signal.Notify(c,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGHUP,
	)
	return c

}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// A dumb function that returns the minimum of two ints
// with the restriction that the result will try to be bigger than
// a third given value.
func cappedMin(a, b, c int) int {
	if a < b && a > c {
		return a
	}
	if b < a && b > c {
		return b
	}
	return max(a, b)
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
	log.Debug("Conf", "Parsed as", conf)
	return conf
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

func dumpCardCacheTimer(conf *configuration, cache *lru.ARCCache) {
	for {
		// Override for Dev
		if conf.DevMode {
			log.Debug("Dumping cache, sleeping (short)")
			time.Sleep(30 * time.Second)
		} else {
			log.Debug("Dumping cache, sleeping (long)")
			time.Sleep(cacheDumpTimer)
		}
		if err := dumpCardCache(conf, cache); err != nil {
			log.Warn("Dump card cache timer", "Error", err)
		}
	}
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

func deleteItemFromCache(key string) error {
	if nameToCardCache.Contains(key) {
		nameToCardCache.Remove(key)
		return nil
	}
	return fmt.Errorf("Key not found")
}

func formatMessageForIRC(input string) string {
	input = strings.Replace(input, "<b>", "\x02", -1)
	input = strings.Replace(input, "</b>", "\x0F", -1)
	input = strings.Replace(input, "<i>", "\x1D", -1)
	input = strings.Replace(input, "</i>", "\x0F", -1)
	return input
}

func formatMessageForSlack(input string) string {
	input = strings.Replace(input, "<b>", "*", -1)
	input = strings.Replace(input, "</b>", "*", -1)
	input = strings.Replace(input, "<i>", "_", -1)
	input = strings.Replace(input, "</i>", "_", -1)
	return input
}

func goRoutines() interface{} {
	return runtime.NumGoroutine()
}

func handleICC(cardTokens []string) string {
	return fmt.Sprintf("%cce %crown %citadel", strings.ToUpper(cardTokens[0])[0], strings.ToUpper(cardTokens[1])[0], strings.ToUpper(cardTokens[2])[0])
}

// Null Coalescing Operator
func nco(a string, b string) string {
	if a != "" {
		return a
	}
	return b
}

func mergeIntStringMaps(new map[int]string, existing map[int]string) map[int]string {
	for k, v := range new {
		existing[k] = v
	}
	return existing
}
