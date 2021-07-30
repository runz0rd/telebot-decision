package decision

import (
	"crypto/md5"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"gopkg.in/tucnak/telebot.v2"
)

type HandlerConfig struct {
	What           string
	Options        []string
	OptionsPerPage int
}

type TelegramDecisionHandler interface {
	// register a telegram endpoint that will call the Trigger
	TelegramHandle() (endpoint interface{})

	// entrypoint; produce a what and options
	Trigger(args ...interface{}) (what string, options []string, err error)

	// handle the decision result and produce a reply message
	OnDecision(what string, options []string, index int) (reply string, err error)

	// handle cancelled decision and produce a reply message
	OnCancel() (reply string, err error)

	// handle error and produce a reply message
	OnError(err error) (reply string)

	// get handler configuration
	Configuration() *HandlerConfig
}

type TelegramEventDecision struct {
	tb             *telebot.Bot
	telegramUserId int
}

func NewTelegramDecisionWithHandler(tb *telebot.Bot, telegramUserId int) *TelegramEventDecision {
	return &TelegramEventDecision{tb, telegramUserId}
}

func (td *TelegramEventDecision) Handle(hs ...TelegramDecisionHandler) {
	for _, h := range hs {
		telegramEndpoint := h.TelegramHandle()
		if telegramEndpoint != nil {
			td.tb.Handle(telegramEndpoint, func(m *telebot.Message) {
				td.handleTrigger(m, h)
			})
		} else {
			td.handleTrigger(&telebot.Message{Sender: &telebot.User{ID: td.telegramUserId}}, h)
		}
	}
}

func (td *TelegramEventDecision) handleTrigger(m *telebot.Message, h TelegramDecisionHandler) {
	var err error
	h.Configuration().What, h.Configuration().Options, err = h.Trigger()
	if err != nil {
		td.handleError(m, h, err)
	}
	td.handleMessage(m, h.Configuration().What, td.createReplyMarkup(h))
}

func (td *TelegramEventDecision) createReplyMarkup(h TelegramDecisionHandler) *telebot.ReplyMarkup {
	rm := &telebot.ReplyMarkup{}
	var pages []Page
	var page Page
	var pageCount int
	if h.Configuration().OptionsPerPage == 0 {
		h.Configuration().OptionsPerPage = 10
	}
	for i, option := range h.Configuration().Options {
		page = append(page, rm.Row(td.optionButton(option, i, rm, h)))
		if (i+1)%h.Configuration().OptionsPerPage == 0 || i+1 == len(h.Configuration().Options) {
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
	hash := fmt.Sprintf("%x", md5.Sum([]byte(h.Configuration().What+option)))
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
	msg, err := h.OnDecision(h.Configuration().What, h.Configuration().Options, decisionIndex)
	if err != nil {
		return errors.Wrap(err, "handler error: %v")
	}
	td.handleMessage(c.Message.ReplyTo, msg, true)
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
		td.handleMessage(c.Message.ReplyTo, msg, true)
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

func (td *TelegramEventDecision) handleMessage(m *telebot.Message, message string, options ...interface{}) {
	if message == "" {
		return
	}
	if m.Chat != nil {
		_, _ = td.tb.Reply(m, message, options)
	} else {
		_, _ = td.tb.Send(&telebot.User{ID: m.Sender.ID}, message, options)
	}
}

func (td *TelegramEventDecision) handleError(m *telebot.Message, h TelegramDecisionHandler, err error) {
	text := h.OnError(err)
	if m.Chat != nil {
		_, _ = td.tb.Reply(m, text)
	} else {
		_, _ = td.tb.Send(&telebot.User{ID: m.Sender.ID}, text)
	}
}
