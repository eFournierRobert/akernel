// Package service defines business-logic components and typed sentinel errors.
package service

import "errors"

// ErrModelNotFound indicates that the LLM model requested by the user was not found in the configuration.
var ErrModelNotFound = errors.New("given model not found")

// ErrInvalidConversation indicates a request references a conversation ID out of bounds or nonexistent.
var ErrInvalidConversation = errors.New("invalid conversation")

// ErrConversationDoesNotExist indicates that no conversation could be found for the given ID.
var ErrConversationDoesNotExist = errors.New("conversation does not exist")
