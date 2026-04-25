// Package model defines shared data types used across MAClawBot components.
// These types represent the iLink Bot API message protocol and are used by
// both the ilink HTTP client and the router package.
package model

// iLink message type constants
const (
	MessageTypeText  = 1 // Text message
	MessageTypeImage = 2 // Image message
	MessageTypeVoice = 3 // Voice message (with transcription)
	MessageTypeVideo = 4 // Video message
	MessageTypeFile  = 5 // File message
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
	MD5      string `json:"md5"`      // File MD5
	Size     int64  `json:"size"`     // File size in bytes
	Width    int    `json:"width"`    // Video width
	Height   int    `json:"height"`   // Video height
	Duration int    `json:"duration"` // Duration in seconds
	AesKey   string `json:"aeskey"`   // AES-128-ECB encryption key
}

// FileItem represents a file message.
type FileItem struct {
	MD5      string `json:"md5"`       // File MD5
	Size     int64  `json:"size"`      // File size in bytes
	FileName string `json:"file_name"` // File name
	AesKey   string `json:"aeskey"`    // AES-128-ECB encryption key
}

// Item is a union type for different message content types.
type Item struct {
	Type      int        `json:"type"`                 // Message type (1=text, 2=image, 3=voice, 4=video, 5=file)
	TextItem  *TextItem  `json:"text_item,omitempty"`  // Text content, if type==1
	ImageItem *ImageItem `json:"image_item,omitempty"` // Image content, if type==2
	VoiceItem *VoiceItem `json:"voice_item,omitempty"` // Voice content, if type==3
	VideoItem *VideoItem `json:"video_item,omitempty"` // Video content, if type==4
	FileItem  *FileItem  `json:"file_item,omitempty"`  // File content, if type==5
}

// Message represents an incoming message from iLink.
type Message struct {
	FromUserID   string `json:"from_user_id"`            // Sender's user ID
	ToUserID     string `json:"to_user_id"`              // Recipient's user ID (bot)
	ContextToken string `json:"context_token,omitempty"` // Context for replies
	MessageType  int    `json:"message_type"`            // Message type (1=incoming)
	ClientID     string `json:"client_id,omitempty"`     // Client identifier
	ItemList     []Item `json:"item_list"`               // Message content items
}

// SendMessage represents an outgoing message to iLink.
type SendMessage struct {
	FromUserID   string `json:"from_user_id"`
	ToUserID     string `json:"to_user_id"`
	ClientID     string `json:"client_id"`
	MessageType  int    `json:"message_type"`  // 2=outgoing
	MessageState int    `json:"message_state"` // 2=sent
	ItemList     []Item `json:"item_list"`
	ContextToken string `json:"context_token,omitempty"`
}
