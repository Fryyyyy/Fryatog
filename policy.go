package main

const mtrURL = "https://blogs.magicjudges.org/rules/mtr"
const ipgURL = "https://blogs.magicjudges.org/rules/ipg"
const rulesURL = "http://cr.mtgipg.com"
const jarURL = "http://jar.mtgipg.com"

func handlePolicyQuery(input []string) string {
	if input[0] == "url" {
		return handlePolicyQuery(input[1:])
	}

	if input[0] == "jar" {
		return jarURL
	}

	if input[0] == "cr" {
		return rulesURL
	}

	if input[0] == "mtr" {
		if len(input) == 1 {
			return rulesURL
		}
		replaced := policyRegex.ReplaceAllString(input[1], "-")
		wellFormedMtrURL := mtrURL + replaced
		return wellFormedMtrURL
	}

	if input[0] == "ipg" {
		if len(input) == 1 {
			return ipgURL
		}
		replaced := policyRegex.ReplaceAllString(input[1], "-")
		wellFormedIpgURL := ipgURL + replaced
		return wellFormedIpgURL
	}
	return ""
}
