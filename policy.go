package main

// import (
// 	"errors"
// 	"flag"
// 	"fmt"
// 	"os"
// 	"strconv"
// 	"strings"
// 	"syscall"
// 	"time"

// 	raven "github.com/getsentry/raven-go"
// 	lru "github.com/hashicorp/golang-lru"
// 	"github.com/nlopes/slack"
// 	cache "github.com/patrickmn/go-cache"
// 	fuzzy "github.com/paul-mannino/go-fuzzywuzzy"
// 	hbot "github.com/whyrusleeping/hellabot"
// 	log "gopkg.in/inconshreveable/log15.v2"
// )

//These *should* be permanent link locations.
const MtrUrl = "https://blogs.magicjudges.org/rules/mtr"
const IpgUrl = "https://blogs.magicjudges.org/rules/ipg"

func HandlePolicyQuery(input []string) (string) {

	if input[0] == "mtr" {
		if len(input) == 1 {
			return MtrUrl
		}
		replaced := policyRegex.ReplaceAllString(input[1], "-")
		wellFormedMtrUrl := MtrUrl + replaced
		return wellFormedMtrUrl
	}

	if input[0] == "ipg" {
		if len(input) == 1 {
			return IpgUrl
		}
		replaced := policyRegex.ReplaceAllString(input[1], "-")
		wellFormedIpgUrl := IpgUrl + replaced
		return wellFormedIpgUrl
	}
	return "Requested policy link not found."
}