package decision

import (
	"log"
	"time"

	"gopkg.in/tucnak/telebot.v2"
)

func NewTelebot(botToken string) (*telebot.Bot, error) {
	tb, err := telebot.NewBot(telebot.Settings{
		Token:     botToken,
		Poller:    &telebot.LongPoller{Timeout: 10 * time.Second},
		ParseMode: telebot.ModeMarkdown,
	})
	return tb, err
}

func messageCleanup(tb *telebot.Bot, m *telebot.Message) error {
	return tb.Delete(m)
}

func ReplyError(tb *telebot.Bot, to *telebot.Message, err error) {
	log.Println(err)
	_, _ = tb.Reply(to, err.Error())
}

func SendError(tb *telebot.Bot, userId int, err error) {
	log.Println(err)
	_, _ = tb.Send(&telebot.User{ID: userId}, err.Error())
}

func SpaceFiller(s string, space int) string {
	var filler string
	for i := 0; i < space-len(s); i++ {
		filler += " "
	}
	return filler
}
