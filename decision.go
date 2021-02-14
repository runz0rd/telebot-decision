package decision

import (
	"crypto/md5"
	"fmt"
	"log"
	"strconv"

	"gopkg.in/tucnak/telebot.v2"
)

type Page []telebot.Row

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
	perPage     int
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
		decided: make(chan bool),
		perPage: 10}
}

func (td *TelegramDecision) PerPage(num int) *TelegramDecision {
	td.perPage = num
	return td
}

func (td *TelegramDecision) Messagef(format string, a ...interface{}) *TelegramDecision {
	td.decisionMsg = fmt.Sprintf(format, a...)
	return td
}

func (td *TelegramDecision) Successf(format string, a ...interface{}) *TelegramDecision {
	td.successMsg = fmt.Sprintf(format, a...)
	return td
}

func (td *TelegramDecision) Send(userId int) (bool, error) {
	return td.send(userId)
}

func (td *TelegramDecision) Reply(to *telebot.Message) (bool, error) {
	return td.send(to)
}

func (td *TelegramDecision) send(to interface{}) (bool, error) {
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
	decided := <-td.decided
	log.Printf("decided on %q", td.d.what)

	if td.successMsg == "" || !decided {
		return decided, nil
	}
	switch v := to.(type) {
	case *telebot.Message:
		_, _ = td.tb.Reply(v, td.successMsg)
	case int:
		_, _ = td.tb.Send(&telebot.User{ID: v}, td.successMsg)
	}
	return decided, nil
}

func (td *TelegramDecision) createReplyMarkup(d Decision) *telebot.ReplyMarkup {
	rm := &telebot.ReplyMarkup{}
	var pages []Page
	var page Page
	var pageCount int
	for i, option := range d.options {
		page = append(page, rm.Row(td.optionButton(d, option, i, rm)))
		if (i+1)%td.perPage == 0 || i+1 == len(d.options) {
			pageCount = len(pages) + 1
			prevButton := rm.Data("<", "prev"+fmt.Sprint(pageCount-1), fmt.Sprint(pageCount-1))
			nextButton := rm.Data(">", "next"+fmt.Sprint(pageCount+1), fmt.Sprint(pageCount+1))
			page = append(page, rm.Row(prevButton, nextButton))
			page = append(page, rm.Row(td.cancelButton(rm)))
			pages = append(pages, page)
			td.handlePaginationButton(&prevButton, &pages, rm)
			td.handlePaginationButton(&nextButton, &pages, rm)
			page = Page{}
		}
	}
	rm.Inline(pages[0]...)
	return rm
}

func (td *TelegramDecision) handlePaginationButton(b *telebot.Btn, pages *[]Page, rm *telebot.ReplyMarkup) {
	td.tb.Handle(b, func(c *telebot.Callback) {
		defer td.tb.Respond(c)
		pageIndex, err := strconv.Atoi(c.Data)
		if err != nil || pageIndex < 1 || pageIndex > len(*pages) {
			return
		}
		rm.Inline((*pages)[pageIndex-1]...)
		_, _ = td.tb.Edit(c.Message, c.Message.Text, rm)
	})
}

func (td *TelegramDecision) cancelButton(rm *telebot.ReplyMarkup) telebot.Btn {
	button := rm.Data("cancel", "cancel", "cancel")
	td.tb.Handle(&button, func(c *telebot.Callback) {
		_ = messageCleanup(td.tb, c.Message)
		log.Println("decision cancelled")
		td.decided <- false
	})
	return button
}

func (td *TelegramDecision) optionButton(d Decision, option string, optionIndex int, rm *telebot.ReplyMarkup) telebot.Btn {
	hash := fmt.Sprintf("%x", md5.Sum([]byte(d.what+option)))
	button := rm.Data(option, hash, fmt.Sprint(optionIndex)) // unique and data max is 64 bytes
	td.tb.Handle(&button, func(c *telebot.Callback) {
		defer td.tb.Respond(c)
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
	return button
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
