package telegram

import (
  "context"
  "fmt"
  "main/pkg/utils"
  "time"

  log "github.com/sirupsen/logrus"

  tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot interface {
  Start() error
  StartWithWebhook(link string) error
  SendMessage(m *SendMessage) error
  HandleMessages(handler func(m *Message) error)
}

type bot struct {
  ctx     context.Context
  token   string
  api     *tg.BotAPI
  updates tg.UpdatesChannel
  started chan struct{}
}

func NewBot(ctx context.Context, token string) (Bot, error) {
  api, err := tg.NewBotAPI(token)
  if err != nil {
    return nil, fmt.Errorf("cannot create new bot api: %v", err)
  }
  return &bot{
    ctx: ctx,
    api: api,
  }, nil
}

func (b *bot) Start() error {
  const (
    timeout = 60
  )
  b.updates = b.api.GetUpdatesChan(tg.UpdateConfig{
    Offset:  0,
    Limit:   0,
    Timeout: timeout,
  })
  return nil
}

func (b *bot) StartWithWebhook(link string) error {
  wh, err := tg.NewWebhook(link)
  if err != nil {
    return fmt.Errorf("cannot create new telegram webhook: %v", err)
  }
  if _, err := b.api.Request(wh); err != nil {
    return fmt.Errorf("cannot do request to webhook: %v", err)
  }
  info, err := b.api.GetWebhookInfo()
  if err != nil {
    return fmt.Errorf("cannot get webhook info: %v", err)
  }
  if info.LastErrorDate != 0 {
    return fmt.Errorf("telegram callback failed: %s", info.LastErrorMessage)
  }
  b.updates = b.api.ListenForWebhook("/" + b.token)

  return nil
}

func (b *bot) HandleMessages(handler func(m *Message) error) {
  var (
    msg *tg.Message
    m   *Message
    err error
  )
  for update := range b.updates {
    if msg = update.Message; msg == nil {
      continue
    }
    m = toMessage(msg)

    if err = handler(m); err != nil {
      log.Errorf("cannot handle telegram message: %v", err)
    }
  }
}

func (b *bot) SendMessage(m *SendMessage) error {
  const (
    retryCount = 10
    retryWait  = time.Second
  )
  parseMode := string(m.ParseMode)
  escapedText := tg.EscapeText(string(m.ParseMode), m.Text)
  
  return utils.DoWithRetries(retryCount, retryWait, func() error {
    if _, err := b.api.Send(tg.MessageConfig{
      BaseChat: tg.BaseChat{
        ChatID: m.ChatID,
      },
      Text:      escapedText,
      ParseMode: parseMode,
    }); err != nil {
      return fmt.Errorf("%w: cannot send telegram message: %v", utils.ErrDoRetry, err)
    }
    return nil
  })
}