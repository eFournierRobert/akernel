// Package service defines business-logic components and typed sentinel errors.
package service

import "errors"

// ErrModelNotFound indicates that the Ollama model requested by the user was not found on the server.
var ErrModelNotFound = errors.New("given model not found")

// ErrInvalidConversation indicates a request references a conversation ID out of bounds or nonexistent.
var ErrInvalidConversation = errors.New("invalid conversation")

// ErrConversationDoesNotExist indicates that no conversation could be found for the given ID.
var ErrConversationDoesNotExist = errors.New("conversation does not exist")

var ErrModelDoesNotExist = errors.New("model does not exist")
