package router

import (
	"strings"

	"maclawbot/internal/model"
)

// Re-exported message types from model package for backward compatibility.
// Code should migrate to importing model directly.
type (
	TextItem    = model.TextItem
	VoiceItem   = model.VoiceItem
	ImageItem   = model.ImageItem
	VideoItem   = model.VideoItem
	FileItem    = model.FileItem
	Item        = model.Item
	Message     = model.Message
	SendMessage = model.SendMessage
)

// Re-exported message type constants from model package.
const (
	MessageTypeText  = model.MessageTypeText
	MessageTypeImage = model.MessageTypeImage
	MessageTypeVoice = model.MessageTypeVoice
	MessageTypeVideo = model.MessageTypeVideo
	MessageTypeFile  = model.MessageTypeFile
)

// CmdResult represents the result of processing a command.
type CmdResult struct {
	Text      string // Response text to send back
	IsHandled bool   // true if command was handled, false for passthrough
	Action    string // Optional action trigger: "login" starts bot QR login flow
}

// ExtractText converts a message's item list to plain text.
// Voice messages are converted to "[The user sent a voice message. Here's what they said: "..."]"
func ExtractText(items []Item) string {
	var parts []string
	for _, it := range items {
		switch it.Type {
		case MessageTypeText:
			if it.TextItem != nil && it.TextItem.Text != "" {
				parts = append(parts, it.TextItem.Text)
			}
		case MessageTypeVoice:
			if it.VoiceItem != nil && it.VoiceItem.Text != "" {
				parts = append(parts, "[The user sent a voice message. Here's what they said: \""+it.VoiceItem.Text+"\"]")
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// ProcessCommand handles slash commands and returns the response.
// Returns IsHandled=false for non-command text to enable passthrough to agent.
func ProcessCommand(state *State, text string) CmdResult {
	text = strings.TrimSpace(strings.ToLower(text))

	// Handle /clawbot subcommands
	if strings.HasPrefix(text, "/clawbot") {
		return processClawbotCommand(state, text)
	}

	return CmdResult{IsHandled: false}
}
