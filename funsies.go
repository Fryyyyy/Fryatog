package main

import (
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "gopkg.in/inconshreveable/log15.v2"
)

type Dice struct {
	number   int
	sides    int
	operator byte
	operand  int
}

func (d Dice) calculate() string {
	rand.Seed(time.Now().UnixNano())
	if d.sides == 0 {
		return fmt.Sprintf("1 %d-sided die: %d", d.number, rand.Intn(d.number-1)+1)
	}
	var result int
	for i := 1; i <= d.number; i++ {
		result += rand.Intn(d.sides-1) + 1
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

func RollDice(input string) string {
	diceRegex := regexp.MustCompile(`^(\d+)(?:d?(\d+)([+-]\d+)?)?`)
	inputDice := diceRegex.FindStringSubmatch(input)

	dice, err := validateDice(input, inputDice)
	if err != nil {
		return err.Error()
	}
	log.Debug(fmt.Sprintf("%dd%d%c%d", dice.number, dice.sides, dice.operator, dice.operand))
	return dice.calculate()
}

func validateDice(input string, dice []string) (Dice, error) {

	failure := "roll: Try something like 'roll 4', 'roll 3d8', 'roll 2d6+2'"
	//input didn't pass the regex at all so dice is empty
	if len(dice) == 0 {
		return Dice{}, errors.New(failure)
	}

	//cuts out weirdly formed things
	failCases := []bool{
		dice[2] == "" && len(input) > len(dice[1]),
		strings.Contains(input, "d") && dice[2] == "",
		strings.Contains(input, "-") && dice[3] == "",
		strings.Contains(input, "+") && dice[3] == ""}

	for _, failCase := range failCases {
		if failCase {
			return Dice{}, errors.New(failure)
		}
	}

	var (
		sidesDice int
		operator  byte
		operand   int
	)
	numDice, err := strconv.Atoi(dice[1])
	if err != nil || numDice > 99 {
		return Dice{}, errors.New("That's too many dice. Calm down.")
	}

	if dice[2] != "" {
		sidesDice, err = strconv.Atoi(dice[2])
		if err != nil || sidesDice > 100 {
			return Dice{}, errors.New("When have you ever seen a die with that many sides? Come on.")
		}
		if sidesDice <= 1 {
			return Dice{}, errors.New("Your spherical dice go careening off the flat earth. You know. Those two things that exist.")
		}
	}

	if dice[3] != "" {
		operator = dice[3][0]
		operand, err = strconv.Atoi(dice[3][1:])
		if err != nil || operand > 1000 {
			return Dice{}, errors.New("Haha. No. Why do you need a mod that large?")
		}
	}

	wellFormedDice := Dice{numDice, sidesDice, operator, operand}
	return wellFormedDice, nil
}

func FlipCoin(user string) string {
	coin := []string{"heads", "tails"}
	rand.Seed(time.Now().UnixNano())
	side := coin[rand.Intn(len(coin))]

	return fmt.Sprintf("%s flips a coin: %s.", user, side)
}
