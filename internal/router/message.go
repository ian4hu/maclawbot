package router

import (
	"fmt"
	"strings"
)

// iLink message type constants
const (
	MessageTypeText  = 1  // Text message
	MessageTypeImage = 2  // Image message
	MessageTypeVoice = 3  // Voice message (with transcription)
	MessageTypeVideo = 4  // Video message
	MessageTypeFile  = 5  // File message
)

// TextItem represents the content of a text message.
type TextItem struct {
	Text string `json:"text"`
}

// VoiceItem represents a voice message with iLink transcription.
type VoiceItem struct {
	Text string `json:"text"` // Transcription of the voice message
}

// ImageItem represents an image message.
type ImageItem struct {
	MD5    string `json:"md5"`    // File MD5
	Size   int64  `json:"size"`   // File size in bytes
	Width  int    `json:"width"`  // Image width
	Height int    `json:"height"` // Image height
	AesKey string `json:"aeskey"` // AES-128-ECB encryption key
}

// VideoItem represents a video message.
type VideoItem struct {
	MD5    string `json:"md5"`    // File MD5
	Size   int64  `json:"size"`   // File size in bytes
	Width  int    `json:"width"`  // Video width
	Height int    `json:"height"` // Video height
	Duration int  `json:"duration"` // Duration in seconds
	AesKey string `json:"aeskey"` // AES-128-ECB encryption key
}

// FileItem represents a file message.
type FileItem struct {
	MD5    string `json:"md5"`    // File MD5
	Size   int64  `json:"size"`   // File size in bytes
	FileName string `json:"file_name"` // File name
	AesKey string `json:"aeskey"` // AES-128-ECB encryption key
}

// Item is a union type for different message content types.
type Item struct {
	Type       int        `json:"type"`                 // Message type (1=text, 2=image, 3=voice, 4=video, 5=file)
	TextItem   *TextItem  `json:"text_item,omitempty"`   // Text content, if type==1
	ImageItem  *ImageItem `json:"image_item,omitempty"`  // Image content, if type==2
	VoiceItem  *VoiceItem `json:"voice_item,omitempty"` // Voice content, if type==3
	VideoItem  *VideoItem `json:"video_item,omitempty"` // Video content, if type==4
	FileItem   *FileItem  `json:"file_item,omitempty"`  // File content, if type==5
}

// Message represents an incoming message from iLink.
type Message struct {
	FromUserID    string `json:"from_user_id"`     // Sender's user ID
	ToUserID      string `json:"to_user_id"`       // Recipient's user ID (bot)
	ContextToken  string `json:"context_token,omitempty"` // Context for replies
	MessageType   int    `json:"message_type"`     // Message type (1=incoming)
	ClientID      string `json:"client_id,omitempty"`    // Client identifier
	ItemList      []Item `json:"item_list"`        // Message content items
}

// SendMessage represents an outgoing message to iLink.
type SendMessage struct {
	FromUserID   string `json:"from_user_id"`
	ToUserID     string `json:"to_user_id"`
	ClientID     string `json:"client_id"`
	MessageType  int    `json:"message_type"`     // 2=outgoing
	MessageState int    `json:"message_state"`    // 2=sent
	ItemList     []Item `json:"item_list"`
	ContextToken string `json:"context_token,omitempty"`
}

// CmdResult represents the result of processing a command.
type CmdResult struct {
	Text      string // Response text to send back
	IsHandled bool   // true if command was handled, false for passthrough
}

// ExtractText converts a message's item list to plain text.
// Voice messages are converted to "[The user sent a voice message. Here's what they said: "..."]"
func ExtractText(items []Item) string {
	var parts []string
	for _, it := range items {
		if it.Type == MessageTypeText {
			if it.TextItem != nil && it.TextItem.Text != "" {
				parts = append(parts, it.TextItem.Text)
			}
		} else if it.Type == MessageTypeVoice {
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

// processClawbotCommand handles all /clawbot subcommands:
// help, list, new, set, del, info
func processClawbotCommand(state *State, text string) CmdResult {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		return CmdResult{Text: formatClawbotHelp(), IsHandled: true}
	}

	subcmd := parts[1]

	switch subcmd {
	case "help":
		return CmdResult{Text: formatClawbotHelp(), IsHandled: true}
	case "list":
		return listAgents(state)
	case "set":
		if len(parts) < 3 {
			return CmdResult{Text: "Usage: /clawbot set <agent_name>\nExample: /clawbot set claude", IsHandled: true}
		}
		agentName := parts[2]
		if err := state.SetDefaultAgent(agentName); err != nil {
			return CmdResult{Text: "Error: " + err.Error(), IsHandled: true}
		}
		return CmdResult{Text: fmt.Sprintf("Switched to **%s**.", agentName), IsHandled: true}
	case "new":
		return handleNewAgent(state, parts)
	case "del":
		if len(parts) < 3 {
			return CmdResult{Text: "Usage: /clawbot del <agent_name>\nExample: /clawbot del claude", IsHandled: true}
		}
		agentName := parts[2]
		if err := state.RemoveAgent(agentName); err != nil {
			return CmdResult{Text: "Error: " + err.Error(), IsHandled: true}
		}
		return CmdResult{Text: fmt.Sprintf("Agent **%s** removed.", agentName), IsHandled: true}
	case "info":
		if len(parts) < 3 {
			return CmdResult{Text: formatAgentInfo(state, state.GetDefaultAgent()), IsHandled: true}
		}
		agentName := parts[2]
		return CmdResult{Text: formatAgentInfo(state, agentName), IsHandled: true}
	default:
		return CmdResult{Text: formatClawbotHelp(), IsHandled: true}
	}
}

// handleNewAgent creates a new agent with optional custom tag.
// Syntax: /clawbot new <name> [tag]
// If tag is omitted, defaults to "[Name]"
// If tag contains spaces, wrap in brackets: [Claude Code]
func handleNewAgent(state *State, parts []string) CmdResult {
	if len(parts) < 3 {
		return CmdResult{Text: "Usage: /clawbot new <agent_name> [tag]\nExample: /clawbot new claude\nExample: /clawbot new claude [Claude Code]", IsHandled: true}
	}
	agentName := parts[2]
	port := state.GetNextAvailablePort()

	var tag string
	if len(parts) >= 4 {
		// Check if tag is wrapped in brackets (for multi-word tags)
		if strings.HasPrefix(parts[3], "[") && !strings.HasSuffix(parts[3], "]") {
			// Multi-word tag: find the closing bracket
			endIdx := -1
			for i := 3; i < len(parts); i++ {
				if strings.HasSuffix(parts[i], "]") {
					endIdx = i
					break
				}
			}
			if endIdx == -1 {
				// No closing bracket found, use default
				tag = fmt.Sprintf("[%s]", strings.Title(agentName))
			} else {
				// Extract multi-word tag
				tag = strings.Join(parts[3:endIdx+1], " ")
				tag = strings.TrimPrefix(tag, "[")
				tag = strings.TrimSuffix(tag, "]")
				tag = fmt.Sprintf("[%s]", tag)
			}
		} else {
			// Single-word tag
			tag = fmt.Sprintf("[%s]", parts[3])
		}
	} else {
		tag = fmt.Sprintf("[%s]", strings.Title(agentName))
	}

	agent := Agent{
		Name:    agentName,
		Port:    port,
		Tag:     tag,
		Enabled: true,
	}

	if err := state.AddAgent(agent); err != nil {
		return CmdResult{Text: "Error: " + err.Error(), IsHandled: true}
	}

	return CmdResult{
		Text: fmt.Sprintf("Agent **%s** created on port **%d** (tag: %s).\n\n"+
			"Configure your gateway to use:\n"+
			"```\n"+
			"http://127.0.0.1:%d\n"+
			"```\n"+
			"Run `/clawbot set %s` to switch to this agent.", agentName, port, tag, port, agentName),
		IsHandled: true,
	}
}

// listAgents returns a formatted list of all configured agents.
func listAgents(state *State) CmdResult {
	agents := state.GetAgents()
	if len(agents) == 0 {
		return CmdResult{Text: "No agents configured.", IsHandled: true}
	}

	var lines []string
	lines = append(lines, "**Available Agents:**\n")
	for name, agent := range agents {
		portInfo := fmt.Sprintf("port %d", agent.Port)
		if agent.DefaultAgent {
			portInfo += " (default)"
		}
		lines = append(lines, fmt.Sprintf("- **%s**: %s", name, portInfo))
	}
	lines = append(lines, "\n**Commands:**")
	lines = append(lines, "- `/clawbot new <name>` - Create new agent")
	lines = append(lines, "- `/clawbot set <name>` - Switch to agent")
	lines = append(lines, "- `/clawbot del <name>` - Remove agent")
	lines = append(lines, "- `/clawbot list` - List all agents")

	return CmdResult{Text: strings.Join(lines, "\n"), IsHandled: true}
}

// formatAgentInfo returns detailed information about an agent.
func formatAgentInfo(state *State, agentName string) string {
	agent, ok := state.GetAgent(agentName)
	if !ok {
		return fmt.Sprintf("Agent **%s** not found.", agentName)
	}

	return fmt.Sprintf("**Agent: %s**\n"+
		"- Port: %d\n"+
		"- Tag: %s\n"+
		"- Default: %v",
		agent.Name, agent.Port, agent.Tag, agent.DefaultAgent)
}

// formatClawbotHelp returns the help text for /clawbot commands.
func formatClawbotHelp() string {
	lines := []string{
		"**Clawbot Commands:**",
		"",
		"`/clawbot` - Show this help",
		"`/clawbot list` - List all agents",
		"`/clawbot set <name>` - Switch to agent",
		"`/clawbot new <name> [tag]` - Create new agent",
		"`/clawbot del <name>` - Remove agent",
		"`/clawbot info [name]` - Show agent info",
		"",
		"**Examples:**",
		"- `/clawbot new claude` - Create claude agent with default tag",
		"- `/clawbot new claude [Claude Code]` - Create with custom tag",
		"- `/clawbot set claude` - Switch to claude",
		"- `/clawbot del claude` - Remove claude agent",
	}

	return strings.Join(lines, "\n")
}
