package service

import "errors"

var ErrModelNotFound = errors.New("given model not found")
var ErrInvalidConversation = errors.New("invalid conversation")

var ErrConversationDoesNotExist = errors.New("conversation does not exist")
