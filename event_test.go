package decision

import (
	"testing"

	"gopkg.in/tucnak/telebot.v2"
)

func TestTelegramEventDecision_Handle(t *testing.T) {
	type fields struct {
		tb      *telebot.Bot
		what    string
		options []string
		decided chan bool
		opts    Options
	}
	type args struct {
		h TelegramDecisionHandler
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := &TelegramEventDecision{
				tb:      tt.fields.tb,
				what:    tt.fields.what,
				options: tt.fields.options,
				decided: tt.fields.decided,
				opts:    tt.fields.opts,
			}
			td.Handle(tt.args.h)
		})
	}
}
