package decision

import (
	"crypto/md5"
	"fmt"
	"log"
	"strconv"

	"gopkg.in/tucnak/telebot.v2"
)

type Options struct {
	optionsPerPage int
	useReply       bool
}

type TelegramDecisionHandler interface {
	// handle the bot /start command
	OnStart(senderId int) error

	// handle the what in message and produce a decision message, and options
	OnMessage(message string, senderId int) (string, []string, error)

	// handle the decision result and produce a success message
	OnDecision(what string, options []string, index int) (string, error)

	// handle cancelled decisions
	OnCancel() error

	// handle errors on and between events
	OnError(err error)
}

type TelegramEventDecision struct {
	tb      *telebot.Bot
	what    string
	options []string
	decided chan bool
	opts    Options

	decisionMessage string
	successMessage  string
}

func NewTelegramDecisionWithHandler(tb *telebot.Bot, opts Options) *TelegramEventDecision {
	if opts.optionsPerPage == 0 {
		opts.optionsPerPage = 10
	}
	return &TelegramEventDecision{
		tb:      tb,
		decided: make(chan bool),
		opts:    opts,
	}
}

func (td *TelegramEventDecision) Handle(h TelegramDecisionHandler) {
	var err error
	var message *telebot.Message
	td.tb.Handle("/start", func(m *telebot.Message) {
		if err := h.OnStart(m.Sender.ID); err != nil {
			h.OnError(err)
		}
		message = m
	})
	td.tb.Handle(telebot.OnText, func(m *telebot.Message) {
		td.what = m.Text
		if td.decisionMessage, td.options, err = h.OnMessage(m.Text, m.Sender.ID); err != nil {
			h.OnError(err)
			return
		}
		if td.decisionMessage != "" {
			if err = td.message(m, td.decisionMessage, td.createReplyMarkup(h)); err != nil {
				h.OnError(err)
				return
			}
		}
	})
	log.Printf("waiting on decision for %q", td.what)
	decided := <-td.decided
	log.Printf("decided on %q", td.what)

	if !decided {
		if err := h.OnCancel(); err != nil {
			h.OnError(err)
		}
		return
	}
	if td.successMessage == "" {
		return
	}
	if err = td.message(message, td.decisionMessage); err != nil {
		h.OnError(err)
		return
	}
}

func (td *TelegramEventDecision) createReplyMarkup(h TelegramDecisionHandler) *telebot.ReplyMarkup {
	rm := &telebot.ReplyMarkup{}
	var pages []Page
	var page Page
	var pageCount int
	for i, option := range td.options {
		page = append(page, rm.Row(td.optionButton(option, i, rm, h)))
		if (i+1)%td.opts.optionsPerPage == 0 || i+1 == len(td.options) {
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

func (td *TelegramEventDecision) optionButton(option string, optionIndex int, rm *telebot.ReplyMarkup, h TelegramDecisionHandler) telebot.Btn {
	hash := fmt.Sprintf("%x", md5.Sum([]byte(td.what+option)))
	button := rm.Data(option, hash, fmt.Sprint(optionIndex)) // unique and data max is 64 bytes
	td.tb.Handle(&button, func(c *telebot.Callback) {
		defer td.tb.Respond(c)
		if err := td.handleButtonCallback(c, h); err != nil {
			if c.Message.ReplyTo != nil {
				ReplyError(td.tb, c.Message.ReplyTo, err)
			} else {
				SendError(td.tb, c.Sender.ID, fmt.Errorf("`%v`\n%v", td.what, err))
			}
			td.decided <- false
			return
		}
		td.decided <- true
	})
	return button
}

func (td *TelegramEventDecision) handleButtonCallback(c *telebot.Callback, h TelegramDecisionHandler) error {
	defer messageCleanup(td.tb, c.Message)
	decisionIndex, err := strconv.Atoi(c.Data)
	if err != nil {
		return err
	}
	if td.successMessage, err = h.OnDecision(td.what, td.options, decisionIndex); err != nil {
		return fmt.Errorf("handler error: %v", err)
	}
	return nil
}

func (td *TelegramEventDecision) message(receieved *telebot.Message, text string, options ...interface{}) error {
	var err error
	if td.opts.useReply {
		_, err = td.tb.Reply(receieved, text, options)
	} else {
		_, err = td.tb.Send(&telebot.User{ID: receieved.Sender.ID}, text, options)
	}
	return err
}

func (td *TelegramEventDecision) cancelButton(rm *telebot.ReplyMarkup) telebot.Btn {
	button := rm.Data("cancel", "cancel", "cancel")
	td.tb.Handle(&button, func(c *telebot.Callback) {
		_ = messageCleanup(td.tb, c.Message)
		log.Println("decision cancelled")
		td.decided <- false
	})
	return button
}

func (td *TelegramEventDecision) handlePaginationButton(b *telebot.Btn, pages *[]Page, rm *telebot.ReplyMarkup) {
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
