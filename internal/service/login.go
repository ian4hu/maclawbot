package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"maclawbot/internal/ilink"
	"maclawbot/internal/router"
)

const (
	qrPollInterval = 2 * time.Second // Interval between status polls
	qrLoginTimeout = 5 * time.Minute // Max time to wait for QR scan
)

// StartBotLogin initiates the QR code login flow for adding a new bot.
// It runs asynchronously: sends QR code, polls status, and adds bot on confirmation.
//   - baseURL: iLink API base URL
//   - uid: user ID to send status updates to
//   - ctxToken: context token for replies
//   - client: the existing bot's iLink client (for sending status messages to user)
//   - state: shared state (to persist the new bot)
func StartBotLogin(baseURL, uid, ctxToken string, client *ilink.Client, state *router.State) {
	// Create a client without auth token for QR code operations
	qrClient := ilink.NewClient(baseURL, "")

	// Step 1: Get QR code
	qrResp, err := qrClient.GetBotQRCode()
	if err != nil {
		log.Printf("BotLogin: failed to get QR code: %v", err)
		client.SendText(uid, "❌ 获取二维码失败: "+err.Error(), ctxToken)
		return
	}

	// Send QR code image to user
	qrMsg := fmt.Sprintf(
		"🔐 **扫码登录新 Bot**\n\n"+
			"[点此查看二维码](%s)\n\n"+
			"请在微信中扫描二维码完成登录，有效期 5 分钟。",
		qrResp.QRCodeImgContent,
	)
	client.SendText(uid, qrMsg, ctxToken)

	log.Printf("BotLogin: QR code sent to user %s, qrcode=%s", uid, qrResp.QRCode[:minStr(16, len(qrResp.QRCode))])

	// Step 2: Poll for status changes
	ctx, cancel := context.WithTimeout(context.Background(), qrLoginTimeout)
	defer cancel()

	pollTicker := time.NewTicker(qrPollInterval)
	defer pollTicker.Stop()

	lastStatus := ""

	for {
		select {
		case <-ctx.Done():
			client.SendText(uid, "⏰ 二维码已过期，请重新发送 `/clawbot bot login`。", ctxToken)
			return
		case <-pollTicker.C:
		}

		statusResp, err := qrClient.GetQRCodeStatus(qrResp.QRCode)
		if err != nil {
			log.Printf("BotLogin: poll error: %v", err)
			continue
		}

		// Only send updates when status changes
		if statusResp.Status == lastStatus {
			continue
		}
		lastStatus = statusResp.Status

		switch statusResp.Status {
		case "scaned":
			client.SendText(uid, "📱 已扫码，请在微信中确认登录...", ctxToken)
			log.Printf("BotLogin: qrcode=%s scanned", qrResp.QRCode[:minStr(16, len(qrResp.QRCode))])

		case "confirmed":
			botID := statusResp.ILinkBotID
			if botID == "" {
				botID = statusResp.ILinkUserID
			}
			token := statusResp.BotToken
			effectiveBaseURL := statusResp.BaseURL
			if effectiveBaseURL == "" {
				effectiveBaseURL = baseURL
			}

			// Add the new bot to state
			bot := router.Bot{
				AccountID:    botID,
				Token:        token,
				DefaultAgent: "hermes",
				Enabled:      true,
			}
			if err := state.AddBot(bot); err != nil {
				log.Printf("BotLogin: failed to add bot %s: %v", botID, err)
				client.SendText(uid, fmt.Sprintf("✅ 登录成功！但保存配置失败: %v\nBot ID: `%s`\nToken: `%s`\nBaseURL: `%s`", err, botID, maskToken(token), effectiveBaseURL), ctxToken)
				return
			}

			confirmMsg := fmt.Sprintf(
				"✅ **登录成功！**\n\n"+
					"- Bot ID: `%s`\n"+
					"- Token: `%s`\n"+
					"- Base URL: `%s`\n\n"+
					"Bot 已自动启用，默认使用 **hermes** agent。\n"+
					"使用 `/clawbot bot list` 查看所有 Bot。",
				botID, maskToken(token), effectiveBaseURL,
			)
			client.SendText(uid, confirmMsg, ctxToken)
			log.Printf("BotLogin: qrcode=%s confirmed, bot=%s added", qrResp.QRCode[:minStr(16, len(qrResp.QRCode))], botID)
			return

		case "expired":
			client.SendText(uid, "⏰ 二维码已过期，请重新发送 `/clawbot bot login`。", ctxToken)
			log.Printf("BotLogin: qrcode=%s expired", qrResp.QRCode[:minStr(16, len(qrResp.QRCode))])
			return
		}
	}
}

// maskToken masks a token for display, showing first 8 and last 4 chars.
func maskToken(token string) string {
	if len(token) <= 12 {
		return "****"
	}
	return token[:8] + "..." + token[len(token)-4:]
}
