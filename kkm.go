package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/antigloss/go/logger"
	"github.com/kennygrant/sanitize"

	"gopkg.in/telegram-bot-api.v1"

	"github.com/jinzhu/now"
)

type regReplace struct {
	regexp, replace string
}

type card struct {
	clientID, cardID, date string
}

var txt string

func main() {
	// Add accepted time formats
	now.TimeFormats = append(now.TimeFormats, "02/01/2006")
	now.TimeFormats = append(now.TimeFormats, "02-01-2006")
	now.TimeFormats = append(now.TimeFormats, "02.01.2006")

	logger.Init("./log", 10, 2, 2, false)

	fields := []regReplace{
		{"Numer klienta:.+", "Client number: "},
		{"Numer karty KKM:.+", "KKM card number: "},
		{"Cena:.+", "Price: "},
		{"Data początku ważności:.+", "Valid from: "},
		{"Data końca ważności:.+", "Valid till: "},
		{"Data zwrotu:.+", "Return date: "},
		{"Linie miejskie:.+", "City lines: "},
		{"Linie strefowe:.+", "Zone lines: "},
	}

	bObj := botInit("YOUR-TELEGRAM-TOKEN-HERE")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bObj.GetUpdatesChan(u)
	if err != nil {
		logger.Panic("Error! Couldn't get info about new message.")
	}

	for update := range updates {
		switch {
		case isMatched(update.Message.Text, "^/help$"):
			actionHelp(bObj, update.Message.Chat.ID, update.Message.MessageID)
		case isMatched(update.Message.Text, "^/card \\d+ \\d+( \\d+[-/\\\\.]\\d+[-/\\\\.]\\d+)?$"):
			actionCard(bObj, update.Message.Chat.ID, update.Message.MessageID, update.Message.Text, fields)
		default:
			actionDefault(bObj, update.Message.Chat.ID, update.Message.MessageID)
		}
	}
}

func actionHelp(bObj *tgbotapi.BotAPI, chatID, messageID int) {
	txt = "Available commands:\n/card <clientID> <cardID> <date dd/mm/yyyy> — this will provide details about your KKM card. Argument 'date' isn't mandatory (default value - sysdate)."
	sendMessage(bObj, chatID, messageID, txt)
}

func actionCard(bObj *tgbotapi.BotAPI, chatID, messageID int, messageText string, f []regReplace) {
	c, err := parseCardData(messageText, " ")
	if err != "" {
		logger.Warn(err)
		sendMessage(bObj, chatID, messageID, err)
		return
	}

	cardDetails, err := getCardDetails(c, f)
	if err != "" || cardDetails == nil {
		sendMessage(bObj, chatID, messageID, err)
		return
	}

	txt = ""
	for i := range cardDetails {
		txt += cardDetails[i] + "\n"
	}
	sendMessage(bObj, chatID, messageID, txt)
}

func actionDefault(bObj *tgbotapi.BotAPI, chatID, messageID int) {
	txt = "Command not found :(\nTo get a list of available commands enter: '/help'"
	sendMessage(bObj, chatID, messageID, txt)
}

func parseCardData(s, l string) (card, string) {
	c := card{}
	split := strings.Split(s, l)

	c = card{clientID: string(split[1]),
		cardID: string(split[2]),
		date:   ""}

	if len(split) == 3 {
		t := time.Now()
		c.date = string(t.Format("2006-01-02"))
	} else {
		t, err := now.Parse(split[3])
		if err != nil {
			return c, fmt.Sprintf("Error! Can't parse date %s. Accepted formats are dd/mm/yyyy | dd.mm.yyyy | dd-mm-yyyy etc.\n", split[3])
		}
		c.date = string(t.Format("2006-01-02"))
	}

	return c, ""
}

func getCardDetails(c card, f []regReplace) ([]string, string) {
	cardDetails := []string{}

	resp, err := http.Get(fmt.Sprintf("http://www.mpk.krakow.pl/pl/sprawdz-waznosc-biletu/index,1.html?cityCardType=0&dateValidity=%s&identityNumber=%s&cityCardNumber=%s&sprawdz_kkm=Check", c.date, c.clientID, c.cardID))
	if err != nil {
		fmt.Printf("Error! %v / %v \n", resp.Status, err)
		return nil, "Error! Request to mpk.krakow.pl was unsuccessful."
	}
	defer resp.Body.Close()

	page, _ := ioutil.ReadAll(resp.Body)

	for i := range f {
		r := ""
		r = extractByRegexp(page, f[i].regexp)

		if r != "" {
			l, err := splitAndReplace(r, ":", 0, f[i].replace)
			if err != "" {
				return nil, "Error! Can't parse results from mpk.krakow.pl."
			}
			cardDetails = append(cardDetails, l)
		} else {
			return nil, fmt.Sprintf("No valid tickets were found for card %s and date %s.", c.cardID, c.date)
		}
	}

	return cardDetails, ""
}

func extractByRegexp(s []byte, r string) string {
	re, _ := regexp.Compile(r)
	return sanitize.HTML(re.FindString(string(s)))
}

// Split s by l and replace p index of slice with r
func splitAndReplace(s, l string, p int, r string) (string, string) {
	split := strings.Split(s, l)

	if len(split) > p {
		split[p] = r
		res := ""
		for i := range split {
			res += string(split[i])
		}
		return res, ""
	}

	return "", fmt.Sprintf("Error! Can't split %s by %s and replace %d index.", s, l, p)
}

func botInit(token string) *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logger.Panic("Error! Bot initialization is failed.")
	}

	bot.Debug = true
	logger.Info("Authorized on account %s\n", bot.Self.UserName)

	return bot
}

func isMatched(s string, r string) bool {
	matched, _ := regexp.MatchString(r, s)

	if matched {
		return true
	}
	return false
}

func sendMessage(bObj *tgbotapi.BotAPI, chatID int, messageID int, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = messageID
	bObj.Send(msg)
}
