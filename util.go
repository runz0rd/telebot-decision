package decision

import (
	"log"

	"gopkg.in/tucnak/telebot.v2"
)

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
