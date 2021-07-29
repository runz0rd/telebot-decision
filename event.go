package decision

import (
	"crypto/md5"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"gopkg.in/tucnak/telebot.v2"
)

type Options struct {
	OptionsPerPage int
	UseReply       bool
}

type TelegramDecisionHandler interface {
	// handle the bot /start command and produce a reply message
	OnStart(m *telebot.Message) (reply string, err error)

	// handle the what in message and produce a reply message and options
	OnMessage(m *telebot.Message) (reply string, options []string, err error)

	// handle the decision result and produce a reply message
	OnDecision(what string, options []string, index int) (reply string, err error)

	// handle cancelled decision and produce a reply message
	OnCancel() (reply string, err error)

	// handle error and produce a reply message
	OnError(err error) (reply string)
}

type TelegramEventDecision struct {
	tb      *telebot.Bot
	opts    Options
	what    string
	options []string
}

func NewTelegramDecisionWithHandler(tb *telebot.Bot, opts Options) *TelegramEventDecision {
	if opts.OptionsPerPage == 0 {
		opts.OptionsPerPage = 10
	}
	return &TelegramEventDecision{
		tb:   tb,
		opts: opts,
	}
}

func (td *TelegramEventDecision) Handle(h TelegramDecisionHandler) {
	var err error
	td.tb.Handle("/start", func(m *telebot.Message) {
		msg, err := h.OnStart(m)
		if err != nil {
			td.handleError(m, h, err)
			return
		}
		td.handleMessage(m, msg)
	})
	td.tb.Handle(telebot.OnText, func(m *telebot.Message) {
		var msg string
		td.what = m.Text
		msg, td.options, err = h.OnMessage(m)
		if err != nil {
			td.handleError(m, h, err)
			return
		}
		td.handleMessage(m, msg)
	})
}

func (td *TelegramEventDecision) createReplyMarkup(h TelegramDecisionHandler) *telebot.ReplyMarkup {
	rm := &telebot.ReplyMarkup{}
	var pages []Page
	var page Page
	var pageCount int
	for i, option := range td.options {
		page = append(page, rm.Row(td.optionButton(option, i, rm, h)))
		if (i+1)%td.opts.OptionsPerPage == 0 || i+1 == len(td.options) {
			pageCount = len(pages) + 1
			prevButton := rm.Data("<", "prev"+fmt.Sprint(pageCount-1), fmt.Sprint(pageCount-1))
			nextButton := rm.Data(">", "next"+fmt.Sprint(pageCount+1), fmt.Sprint(pageCount+1))
			page = append(page, rm.Row(prevButton, nextButton))
			page = append(page, rm.Row(td.cancelButton(rm, h)))
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
				td.handleError(c.Message.ReplyTo, h, err)
			} else if c.Message.Sender != nil {
				td.handleError(c.Message, h, err)
			}
			return
		}
	})
	return button
}

func (td *TelegramEventDecision) handleButtonCallback(c *telebot.Callback, h TelegramDecisionHandler) error {
	defer messageCleanup(td.tb, c.Message)
	decisionIndex, err := strconv.Atoi(c.Data)
	if err != nil {
		return err
	}
	msg, err := h.OnDecision(td.what, td.options, decisionIndex)
	if err != nil {
		return errors.Wrap(err, "handler error: %v")
	}
	td.handleMessage(c.Message.ReplyTo, msg)
	return nil
}

func (td *TelegramEventDecision) cancelButton(rm *telebot.ReplyMarkup, h TelegramDecisionHandler) telebot.Btn {
	button := rm.Data("cancel", "cancel", "cancel")
	td.tb.Handle(&button, func(c *telebot.Callback) {
		_ = messageCleanup(td.tb, c.Message)
		msg, err := h.OnCancel()
		if err != nil {
			td.handleError(c.Message.ReplyTo, h, err)
			return
		}
		td.handleMessage(c.Message.ReplyTo, msg)
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

func (td *TelegramEventDecision) handleMessage(receieved *telebot.Message, message string) {
	if message == "" {
		return
	}
	if td.opts.UseReply {
		_, _ = td.tb.Reply(receieved, message)
	} else {
		_, _ = td.tb.Send(&telebot.User{ID: receieved.Sender.ID}, message)
	}
}

func (td *TelegramEventDecision) handleError(receieved *telebot.Message, h TelegramDecisionHandler, err error) {
	text := h.OnError(err)
	if td.opts.UseReply {
		_, _ = td.tb.Reply(receieved, text)
	} else {
		_, _ = td.tb.Send(&telebot.User{ID: receieved.Sender.ID}, text)
	}
}
