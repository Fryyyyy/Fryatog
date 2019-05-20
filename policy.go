package main

//These *should* be permanent link locations.
const MtrUrl   = "https://blogs.magicjudges.org/rules/mtr"
const IpgUrl   = "https://blogs.magicjudges.org/rules/ipg"
const RulesUrl = "cr.mtgipg.com"
const JarUrl   = "jar.mtgipg.com"

func HandlePolicyQuery(input []string) (string) {
	if input[0] == "url" {
		return HandlePolicyQuery(input[1:])
	}

	if input[0] == "jar" {
		return JarUrl
	}

	if input[0] == "cr" {
		return RulesUrl
	}

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
	return ""
}