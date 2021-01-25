package decision

import (
	"crypto/md5"
	"fmt"
	"log"
	"strconv"

	"gopkg.in/tucnak/telebot.v2"
)

type Decision struct {
	what          string
	options       []string
	handler       decisionHandler
	decisionIndex int
}

type TelegramDecision struct {
	tb          *telebot.Bot
	d           *Decision
	decisionMsg string
	successMsg  string
	decided     chan bool
}

type decisionHandler func(what string, options []string, decisionIndex int) error

func NewTelegramDecision(tb *telebot.Bot, what string, options []string, h decisionHandler) *TelegramDecision {
	return &TelegramDecision{
		tb: tb,
		d: &Decision{
			what:    what,
			options: options,
			handler: h,
		},
		decided: make(chan bool)}
}

func (td *TelegramDecision) Messagef(format string, a ...interface{}) *TelegramDecision {
	td.decisionMsg = fmt.Sprintf(format, a...)
	return td
}

func (td *TelegramDecision) Successf(format string, a ...interface{}) *TelegramDecision {
	td.successMsg = fmt.Sprintf(format, a...)
	return td
}

func (td *TelegramDecision) Send(userId int) error {
	return td.send(userId)
}

func (td *TelegramDecision) Reply(to *telebot.Message) error {
	return td.send(to)
}

func (td *TelegramDecision) send(to interface{}) error {
	if td.decisionMsg == "" {
		td.decisionMsg = fmt.Sprintf("`%v`", td.d.what)
	}
	switch v := to.(type) {
	case *telebot.Message:
		_, _ = td.tb.Reply(v, td.decisionMsg, td.createReplyMarkup(*td.d))
	case int:
		_, _ = td.tb.Send(&telebot.User{ID: v}, td.decisionMsg, td.createReplyMarkup(*td.d))
	}

	log.Printf("waiting on decision for %q", td.d.what)
	isSuccess := <-td.decided
	log.Printf("decided on %q", td.d.what)

	if td.successMsg == "" || !isSuccess {
		return nil
	}
	switch v := to.(type) {
	case *telebot.Message:
		_, _ = td.tb.Reply(v, td.successMsg)
	case int:
		_, _ = td.tb.Send(&telebot.User{ID: v}, td.successMsg)
	}
	return nil
}

func (td *TelegramDecision) createReplyMarkup(d Decision) *telebot.ReplyMarkup {
	rm := &telebot.ReplyMarkup{ResizeReplyKeyboard: true}
	var rows []telebot.Row
	for i, option := range d.options {
		hash := fmt.Sprintf("%x", md5.Sum([]byte(d.what+option)))
		button := rm.Data(option, hash, fmt.Sprint(i)) // unique and data max is 64 bytes
		td.tb.Handle(&button, func(c *telebot.Callback) {
			if err := td.handleButtonCallback(c, d); err != nil {
				if c.Message.ReplyTo != nil {
					ReplyError(td.tb, c.Message.ReplyTo, err)
				} else {
					SendError(td.tb, c.Sender.ID, fmt.Errorf("`%v`\n%v", d.what, err))
				}
				td.decided <- false
				return
			}
			td.decided <- true
		})
		rows = append(rows, rm.Row(button))
	}
	rm.Inline(rows...)
	return rm
}

func (td *TelegramDecision) handleButtonCallback(c *telebot.Callback, d Decision) error {
	defer messageCleanup(td.tb, c.Message)
	var err error
	d.decisionIndex, err = strconv.Atoi(c.Data)
	if err != nil {
		return err
	}
	if err := d.handler(d.what, d.options, d.decisionIndex); err != nil {
		return fmt.Errorf("handler error: %v", err)
	}
	return nil
}
