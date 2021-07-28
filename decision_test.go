package decision

import (
	"fmt"
	"io/ioutil"
	"testing"

	"gopkg.in/tucnak/telebot.v2"
	"gopkg.in/yaml.v3"
)

type TelegramConfig struct {
	BotToken string `yaml:"bot_token,omitempty"`
	UserId   int    `yaml:"user_id,omitempty"`
}

func loadConfig(file string) (*TelegramConfig, error) {
	var data []byte
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	c := &TelegramConfig{}
	err = yaml.Unmarshal(data, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func TestNewTelegramDecision(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"phoney", false},
	}

	var options []string
	for i := 0; i < 50; i++ {
		options = append(options, fmt.Sprint(i))
	}
	c, _ := loadConfig("test.yaml")
	tb, _ := NewTelebot(c.BotToken)
	td := NewTelegramDecision(tb, "test", options,
		func(what string, options []string, decisionIndex int) error {
			_, _ = tb.Send(&telebot.User{ID: c.UserId}, options[decisionIndex])
			return nil
		}).PerPage(13)
	go tb.Start()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := td.Send(c.UserId); (err != nil) != tt.wantErr {
				t.Errorf("TelegramDecision.Send() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
