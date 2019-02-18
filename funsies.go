package main

import (
    "fmt"
    "math/rand"
    "regexp"
    "strconv"
    "strings"
    "time"

    log "gopkg.in/inconshreveable/log15.v2"
)

func RollDice(input string) string {
    diceRegex := regexp.MustCompile(`^(\d+)(?:d?(\d+)([+-]\d+)?)?`)
    dice := diceRegex.FindStringSubmatch(input)
    //Testing seed
    //rand.Seed(1)
    rand.Seed(time.Now().UnixNano())
    var operator byte
    var modifier int
    var typeDice int
    log.Debug(fmt.Sprintf("%s :-> %#v", input, dice))

    failure := "\x02roll\x0F: Try something like 'roll 4', 'roll 3d8', 'roll 2d6+2'"

    if len(dice) == 0 {
        return failure
    }

    failCases := []bool{
                      dice[2] == "" && len(input) > len(dice[1]),
                      strings.Contains(input, "d") && dice[2] == "",
                      strings.Contains(input, "-") && dice[3] == "",
                      strings.Contains(input, "+") && dice[3] == "",} 

    for _, failCase := range failCases {
        if (failCase) {
        return failure
        }
    }

    numDice, _ := strconv.Atoi(dice[1])

    if dice[2] == "" {
        return fmt.Sprintf("1 %d-sided die: %d", numDice, rand.Intn(numDice-1) + 1)
    }

    if dice[3] != "" {
        operator = dice[3][0]
        modifier, _ = strconv.Atoi(dice[3][1:])
    }

    if dice[2] != "" {
        typeDice, _ = strconv.Atoi(dice[2])
        if typeDice == 1 {
            return "Your spherical dice go careening off the flat earth. You know. Those two things that exist."
        }
        
        var result int

        for i := 1; i <= numDice; i++ {
            result += (rand.Intn(typeDice-1)+1)
        }

        if operator == '+' {
            result += modifier
        }
        if operator == '-' {
            result -= modifier
        }
        return fmt.Sprintf("%d %d-sided dice %s: %d", numDice, typeDice, dice[3], result)
    }
    return ""
}

func FlipCoin(user string) string {
    coin := []string{"heads", "tails",}
    rand.Seed(time.Now().UnixNano())
    side := coin[rand.Intn(len(coin))]

    return fmt.Sprintf("%s flips a coin: \x02%s\x0F.", user, side)
}
