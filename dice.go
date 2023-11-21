package main

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	log "gopkg.in/inconshreveable/log15.v2"
)

// Roll represents a single rolled unit (which may be multiple dice)
type Roll struct {
	number   int
	sides    int
	operator byte
	operand  int
}

func (d Roll) calculate() string {
	if d.sides == 0 {
		return fmt.Sprintf("1 %d-sided die: %d", d.number, rand.Intn(d.number)+1)
	}
	var result int
	for i := 1; i <= d.number; i++ {
		result += rand.Intn(d.sides) + 1
	}

	if d.operand != 0 {
		if d.operator == '+' {
			result += d.operand
		}
		if d.operator == '-' {
			result -= d.operand
		}
		return fmt.Sprintf("%d %d-sided dice (%c%d): %d", d.number, d.sides, d.operator, d.operand, result)
	}
	return fmt.Sprintf("%d %d-sided dice: %d", d.number, d.sides, result)

}

func rollDice(input string) string {
	inputDice := diceRegex.FindStringSubmatch(input)

	dice, err := validateDice(input, inputDice)
	if err != nil {
		return err.Error()
	}
	log.Debug("RollDice", "Input", fmt.Sprintf("%dd%d%c%d", dice.number, dice.sides, dice.operator, dice.operand))
	return dice.calculate()
}

func validateDice(input string, dice []string) (Roll, error) {
	log.Debug("Validate Dice", "Input", input, "Dice array", dice)
	failure := "Try something like '!roll d4', '!roll 3d8', '!roll 2d6+2'"

	// Input didn't pass the regex at all so dice is empty
	if strings.TrimSpace(input) == "" || len(dice) == 0 {
		return Roll{}, errors.New(failure)
	}

	//cuts out weirdly formed things
	failCases := []bool{
		dice[2] == "" && len(input) > len(dice[1]),
		strings.Contains(input, "d") && dice[2] == "",
		strings.Contains(input, "-") && dice[3] == "",
		strings.Contains(input, "+") && dice[3] == ""}

	for _, failCase := range failCases {
		if failCase {
			return Roll{}, errors.New(failure)
		}
	}

	var (
		sidesDice int
		operator  byte
		operand   int
		numDice   = 1
		err       error
	)
	if dice[1] != "" {
		numDice, err = strconv.Atoi(dice[1])
		if err != nil {
			return Roll{}, errors.New("malformed roll")
		}
		if numDice > 99 {
			return Roll{}, errors.New("malformed roll (max dice is 99)")
		}
	}

	if dice[2] != "" {
		sidesDice, err = strconv.Atoi(dice[2])
		if err != nil {
			return Roll{}, errors.New("malformed roll")
		}
		if sidesDice > 100 {
			return Roll{}, errors.New("malformed roll (max sides is 100)")
		}
		if sidesDice <= 1 {
			return Roll{}, errors.New("malformed roll (min sides is 2)")
		}
	}

	if dice[3] != "" {
		operator = dice[3][0]
		operand, err = strconv.Atoi(dice[3][1:])
		if err != nil {
			return Roll{}, errors.New("malformed roll")
		}
		if operand > 1000 {
			return Roll{}, errors.New("malformed roll (max operand is 1000)")
		}
	}

	wellFormedDice := Roll{numDice, sidesDice, operator, operand}
	return wellFormedDice, nil
}

func flipCoin(input string) string {
	coinCount, err := parseCoinCount(input)
	if err != nil {
		return err.Error()
	}

	flips := make([]int, coinCount)
	for i := 0; i < coinCount; i++ {
		flips[i] = rand.Intn(2)
	}
	return formatFlips(flips, coinCount)
}

func parseCoinCount(input string) (int, error) {
	coinMaximum := 50

	coins := coinRegex.FindStringSubmatch(input)
	if coins[1] == "" {
		// no number specified - implicit one flip
		return 1, nil
	}

	numCoins, err := strconv.Atoi(coins[1])
	if err != nil {
		return 0, errors.New("malformed coin toss")
	}

	if numCoins > coinMaximum {
		return 0, errors.New("malformed coin toss (max count is 50)")
	}
	if numCoins < 1 {
		return 0, errors.New("malformed coin toss (min count is 1)")
	}
	return numCoins, nil
}

func formatFlips(flips []int, count int) string {
	prefix := ""
	coinNames := []string{"Heads", "Tails"}
	joiner := ", "
	if count > 5 {
		prefix = fmt.Sprintf("%d coins: ", count)
		coinNames = []string{"H", "T"}
		joiner = ""
	}

	formatted := make([]string, len(flips))
	for i := 0; i < len(formatted); i++ {
		formatted[i] = coinNames[flips[i]]
	}

	return fmt.Sprintf("%s%s.", prefix, strings.Join(formatted, joiner))
}


