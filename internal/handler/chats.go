package handler

import (
	"context"
	"fmt"
	"main/internal/chats"
	"main/internal/model"
	"main/pkg/http"
	"main/pkg/telegram"
	"main/pkg/tree"
	"main/pkg/utils"
	"strings"
)

func (h *Handler) setChatsTrees() {
	h.chatsTrees = chats.NewChatsTrees(func() tree.Tree[chats.Link, *chats.State] {
		root := tree.NewTree[chats.Link, *chats.State]()

		// node for /start
		root.Push("start", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// if previous message id is set
				if entity := root.Next("start").Entity(); entity != nil && entity.MessageID != 0 {
					// edit previous message to start
					messageID, err = h.bot.EditMessage(newStartMessage(input.ChatID).ToEditMessage(entity.MessageID))
				} else {
					// else send start message
					messageID, err = h.bot.SendMessage(newStartMessage(input.ChatID))
				}
				// put chat id to pending chats
				h.pendingChats.Put(input.ChatID)

				// return message id and error
				return messageID, err
			},
		})

		// go to child node /start
		start := root.Next("start")

		// node for /man
		start.Push("man", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// got previous message id
				prevID := start.Entity().MessageID
				// edit previous message to sub
				return h.bot.EditMessage(newManMessage(input.ChatID).ToEditMessage(prevID))
			},
		})

		// node for /sub
		start.Push("sub", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// got previous message id
				prevID := start.Entity().MessageID
				// edit previous message to sub
				return h.bot.EditMessage(newSubMessage(input.ChatID).ToEditMessage(prevID))
			},
		})

		// node for /unsub
		start.Push("unsub", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// got previous message id
				prevID := start.Entity().MessageID

				// try got sub id from query
				if subID := http.MustParseQuery(input.Command).Get("id"); subID != "" {
					// delete user subscription by id
					if err := h.storage.DeleteUserSubscription(input.Ctx, subID); err != nil {
						return 0, err
					}
					return h.bot.EditMessage(newUnsubCompleteMessage(input.ChatID).ToEditMessage(prevID))
				}
				// got subscriptions from storage for user
				subs, err := h.storage.UserSubscriptions(input.Ctx, input.UserID)
				if err != nil {
					return 0, err
				}
				return h.bot.EditMessage(newUnsubMessage(input.ChatID, subs).ToEditMessage(prevID))
			},
		})

		// node for /contacts
		start.Push("contacts", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// got previous message id
				prevID := start.Entity().MessageID
				// edit previous message to contacts
				return h.bot.EditMessage(newContactsMessage(input.ChatID).ToEditMessage(prevID))
			},
		})
		// do not push node for /back

		// go to child node /sub
		sub := start.Next("sub")

		// node for /area
		sub.Push("area", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// try got area id from query
				if areaID := http.MustParseQuery(input.Command).Get("id"); areaID != "" {
					// set area id for user vacancy
					subVac := h.subVacancies.GetPut(input.ChatID, &subVacancy{})
					subVac.Area = areaID

					// got previous message id
					prevID := sub.Entity().MessageID

					// if sub vac completely filled
					if subVac.IsFilled() {
						// edit previous message to area
						return h.bot.EditMessage(newConfirmCancelMessage(input.ChatID).ToEditMessage(prevID))
					}
					// else edit previous message to fill fields
					return h.bot.EditMessage(newFillFieldsMessage(input.ChatID).ToEditMessage(prevID))
				}
				// got previous message id
				prevID := sub.Entity().MessageID
				// edit previous message to area
				return h.bot.EditMessage(newAreaMessage(input.ChatID).ToEditMessage(prevID))
			},
		})

		// node for /experience
		sub.Push("experience", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// try got area id from query
				if experienceID := http.MustParseQuery(input.Command).Get("id"); experienceID != "" {
					// set experience id for user vacancy
					subVac := h.subVacancies.GetPut(input.ChatID, &subVacancy{})
					subVac.Experience = experienceID

					// got previous message id
					prevID := sub.Entity().MessageID

					// if sub vac completely filled
					if subVac.IsFilled() {
						// edit previous message to area
						return h.bot.EditMessage(newConfirmCancelMessage(input.ChatID).ToEditMessage(prevID))
					}
					// else edit previous message to fill fields
					return h.bot.EditMessage(newFillFieldsMessage(input.ChatID).ToEditMessage(prevID))
				}
				// got previous message id
				prevID := sub.Entity().MessageID
				// edit previous message to area
				return h.bot.EditMessage(newExperienceMessage(input.ChatID).ToEditMessage(prevID))
			},
		})

		// node for /keywords
		sub.Push("keywords", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// got previous message id
				prevID := sub.Entity().MessageID
				// edit previous message to area
				return h.bot.EditMessage(newKeywordsMessage(input.ChatID).ToEditMessage(prevID))
			},
		})

		// go to child node /area
		area := sub.Next("area")

		// node for area /confirm
		area.Push("confirm", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// got previous message id
				prevID := sub.Entity().MessageID
				// edit previous message to confirm
				if messageID, err = h.bot.EditMessage(newConfirmMessage(input.ChatID).ToEditMessage(prevID)); err != nil {
					return 0, err
				}
				// create new task for put subscription to storage
				h.newTaskPutSubscription(input.UserID, input.ChatID)

				return messageID, nil
			},
		})

		// node for experience /cancel
		area.Push("cancel", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// got previous message id
				prevID := sub.Entity().MessageID
				// edit previous message to area
				if messageID, err = h.bot.EditMessage(newCancelMessage(input.ChatID).ToEditMessage(prevID)); err != nil {
					return 0, err
				}
				return messageID, nil
			},
		})

		// go to child node /experience
		experience := sub.Next("experience")

		// node for experience /confirm
		experience.Push("confirm", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// got previous message id
				prevID := sub.Entity().MessageID
				// edit previous message to confirm
				if messageID, err = h.bot.EditMessage(newConfirmMessage(input.ChatID).ToEditMessage(prevID)); err != nil {
					return 0, err
				}
				// create new task for put subscription to storage
				h.newTaskPutSubscription(input.UserID, input.ChatID)
				// create new chat tree for chat id
				h.chatsTrees.NewTree(input.ChatID)

				return messageID, nil
			},
		})

		// node for experience /cancel
		experience.Push("cancel", &chats.State{
			Event: func(input *chats.EventInput) (messageID int64, err error) {
				// got previous message id
				prevID := sub.Entity().MessageID
				// edit previous message to area
				if messageID, err = h.bot.EditMessage(newCancelMessage(input.ChatID).ToEditMessage(prevID)); err != nil {
					return 0, err
				}
				return messageID, nil
			},
		})

		// return root as full pushed chat tree
		return root
	})
}

func (h *Handler) HandleMessages(ctx context.Context, m *telegram.Message) error {
	// handle text messages
	if m.IsText() {
		chatTree := h.chatsTrees.Tree(m.ChatID)

		if link := chatTree.Link(); link == "keywords" {
			// set experience id for user vacancy
			subVac := h.subVacancies.GetPut(m.ChatID, &subVacancy{})
			subVac.Keywords = m.Text

			if entity := chatTree.Entity(); entity != nil {
				// got previous message id
				prevID := entity.MessageID

				// if user vacancy completely filled
				if subVac.IsFilled() {
					// edit previous message to confirm
					messageID, err := h.bot.EditMessage(newConfirmCancelMessage(m.ChatID).ToEditMessage(prevID))
					if err != nil {
						return err
					}
					entity.MessageID = messageID

					// push node for /confirm
					chatTree.Push("confirm", &chats.State{
						Event: func(input *chats.EventInput) (messageID int64, err error) {
							// edit previous message to confirm
							if messageID, err = h.bot.EditMessage(newConfirmMessage(input.ChatID).ToEditMessage(prevID)); err != nil {
								return 0, err
							}
							// create new task for put subscription to storage
							h.newTaskPutSubscription(input.UserID, input.ChatID)

							return messageID, nil
						},
					})

					// push node for /cancel
					chatTree.Push("cancel", &chats.State{
						Event: func(input *chats.EventInput) (messageID int64, err error) {
							// edit previous message to area
							if messageID, err = h.bot.EditMessage(newCancelMessage(input.ChatID).ToEditMessage(prevID)); err != nil {
								return 0, err
							}
							return messageID, nil
						},
					})

					return nil
				}
				// else edit message to fill fields
				messageID, err := h.bot.EditMessage(newFillFieldsMessage(m.ChatID).ToEditMessage(prevID))
				if err != nil {
					return err
				}
				entity.MessageID = messageID
			}
		}
		return nil
	}
	// handle command messages
	if m.IsCommand() {
		link := chats.Link(m.Command)
		chatTree := h.chatsTrees.Tree(m.ChatID)

		defer func() {
			// if link it /confirm or /cancel
			if utils.OneOf(func(s string) bool {
				return s == string(link)
			}, "confirm", "cancel", "stop") {

				// delete chat state
				h.deleteChatState(m.ChatID)
				// create new chat tree for chat id
				h.chatsTrees.NewTree(m.ChatID)
			}
		}()

		// handle /back
		if link == "back" {
			chatTree = chatTree.Prev()

			if entity := chatTree.Entity(); entity != nil {
				messageID, err := entity.Event(&chats.EventInput{
					Ctx:     ctx,
					UserID:  m.UserID,
					ChatID:  m.ChatID,
					Text:    m.Text,
					Command: m.Command,
				})
				if err != nil {
					return err
				}
				h.chatsTrees.SetTree(m.ChatID, chatTree)

				entity.MessageID = messageID
			}
			return nil
		}

		// if link it /area, /experience, /sub
		if utils.OneOf(func(s string) bool {
			return strings.HasPrefix(string(link), s)
		}, "area", "experience", "unsub") {

			// if link has query suffix
			if http.HasQuery(string(link)) {
				chatTree = chatTree.Prev()
			}
			// trim query suffixes for /area and /experience links
			link = chats.Link(http.TrimQuery(string(link)))
		}
		chatTree = chatTree.Next(link)

		if entity := chatTree.Entity(); entity != nil {
			messageID, err := entity.Event(&chats.EventInput{
				Ctx:     ctx,
				UserID:  m.UserID,
				ChatID:  m.ChatID,
				Text:    m.Text,
				Command: m.Command,
			})
			if err != nil {
				return err
			}
			entity.MessageID = messageID
			h.chatsTrees.SetTree(m.ChatID, chatTree)
		}
	}

	return nil
}

func (h *Handler) newTaskPutSubscription(userID, chatID int64) {
	// got subscription vacancy for user
	subVac := h.subVacancies.GetPut(chatID, &subVacancy{})

	// push task to queue
	h.taskQueue.Push(func() error {
		if err := h.storage.PutUserSubscription(h.ctx, &model.Subscription{
			SubscriptionID: utils.NewUUID(),
			UserID:         userID,
			ChatID:         chatID,
			Keywords:       subVac.Keywords,
			Area:           subVac.Area,
			Experience:     subVac.Experience,
			CreatedAt:      utils.NowTimeUTC(),
		}); err != nil {
			return fmt.Errorf("cannot put subscription in storage: %v", err)
		}
		return nil
	})
}

func (h *Handler) deleteChatState(chatID int64) {
	// remove chat from
	h.pendingChats.Delete(chatID)

	// delete subscription vacancy for user
	h.subVacancies.Delete(chatID)
}