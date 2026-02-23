// Package webhook handles daily webhook functionality for sending anime pictures
package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"KawaiiBot/api"
)

// DailyWebhook handles the daily webhook functionality
type DailyWebhook struct {
	webhookURL string
	nekosAPI   *api.Client
	waifuAPI   *api.WaifuClient
	enabled    bool
	mutex      sync.RWMutex
	lastSent   time.Time
}

// New creates a new DailyWebhook instance
func New(nekosAPI *api.Client, waifuAPI *api.WaifuClient) *DailyWebhook {
	webhookURL := os.Getenv("WEBHOOK_URL")

	// Validate webhook URL format if provided
	if webhookURL != "" && !isValidDiscordWebhookURL(webhookURL) {
		log.Printf("[WEBHOOK] Warning: WEBHOOK_URL does not appear to be a valid Discord webhook URL: %s", webhookURL)
	}

	dw := &DailyWebhook{
		webhookURL: webhookURL,
		nekosAPI:   nekosAPI,
		//		waifuAPI:   waifuAPI,
		enabled: true,
	}

	return dw
}

// IsEnabled returns whether the daily webhook is enabled
func (dw *DailyWebhook) IsEnabled() bool {
	dw.mutex.RLock()
	defer dw.mutex.RUnlock()
	return dw.enabled && dw.webhookURL != ""
}

// SetEnabled sets the enabled status of the daily webhook
func (dw *DailyWebhook) SetEnabled(enabled bool) {
	dw.mutex.Lock()
	defer dw.mutex.Unlock()
	dw.enabled = enabled
	log.Printf("WebHook %t", dw.enabled)
}

// Toggle toggles the enabled status of the daily webhook
func (dw *DailyWebhook) Toggle() bool {
	dw.mutex.Lock()
	defer dw.mutex.Unlock()
	dw.enabled = !dw.enabled
	log.Printf("changed Webhook to %t", dw.enabled)
	return dw.enabled
}

// GetStatus returns the current status of the daily webhook
func (dw *DailyWebhook) GetStatus() (enabled bool, url string) {
	dw.mutex.RLock()
	defer dw.mutex.RUnlock()
	return dw.enabled, dw.webhookURL
}

// WebhookPayload represents the JSON payload for Discord webhook
type WebhookPayload struct {
	Content string         `json:"content"`
	Embeds  []WebhookEmbed `json:"embeds,omitempty"`
}

// WebhookEmbed represents an embed in the webhook payload
type WebhookEmbed struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Image       *Image `json:"image,omitempty"`
	Color       int    `json:"color,omitempty"`
}

// Image represents an image in an embed
type Image struct {
	URL string `json:"url"`
}

// SendDailyWebhook sends the daily webhook with waifu and catgirl pictures
func (dw *DailyWebhook) SendDailyWebhook() error {
	if !dw.IsEnabled() {
		return fmt.Errorf("daily webhook is disabled")
	}

	log.Println("[WEBHOOK] Starting daily webhook send process...")
	log.Printf("[WEBHOOK] Webhook URL: %s", dw.webhookURL)

	// Always get random content (mixed SFW/NSFW) by not specifying NSFW preference
	log.Println("[WEBHOOK] Fetching random waifu image...")

	// For waifu.im, we'll try passing false as a neutral option to get mixed results
	/*	waifuImages, err := dw.waifuAPI.GetWaifuImages(1, false, false)
		if err != nil {
			return fmt.Errorf("failed to fetch waifu image: %w", err)
		}
		log.Printf("[WEBHOOK] Fetched %d waifu images", len(waifuImages))

		// Validate waifu image URLs
		if len(waifuImages) > 0 {
			log.Printf("[WEBHOOK] Waifu image details: ID=%d, URL=%s, Extension=%s, NSFW=%t",
				waifuImages[0].ImageID, waifuImages[0].URL, waifuImages[0].Extension, waifuImages[0].IsNSFW)
		}
	*/
	// Fetch one catgirl image - use empty rating to get random mixed content
	log.Println("[WEBHOOK] Fetching random catgirl image...")
	catgirlImages, err := dw.nekosAPI.GetRandomImages(1, "")
	if err != nil {
		return fmt.Errorf("failed to fetch catgirl image: %w", err)
	}
	log.Printf("[WEBHOOK] Fetched %d catgirl images", len(catgirlImages))

	// Validate catgirl image URLs
	if len(catgirlImages) > 0 {
		catgirlURL := fmt.Sprintf("https://nekos.moe/image/%s.jpg", catgirlImages[0].ID)
		log.Printf("[WEBHOOK] Catgirl image details: ID=%s, Constructed URL=%s, NSFW=%t",
			catgirlImages[0].ID, catgirlURL, catgirlImages[0].NSFW)
	}

	// Build content with fallback URLs in case embeds fail
	content := "## 🌸 Your daily motivational waifu/catgirl 🌸\n*Starting your day with some kawaii energy!* 💕\n🎲 *Today's random selection!* 🎲"

	// Add direct URLs to content as fallback
	//	if len(waifuImages) > 0 {
	//		content += fmt.Sprintf("\n\n**💜 Daily Waifu:** %s", waifuImages[0].URL)
	//	}
	if len(catgirlImages) > 0 {
		content += fmt.Sprintf("\n**🐱 Daily Catgirl:** https://nekos.moe/image/%s.jpg", catgirlImages[0].ID)
	}

	// Create webhook payload with both embeds and fallback URLs
	payload := WebhookPayload{
		Content: content,
		Embeds:  []WebhookEmbed{},
	}

	// Add waifu embed if we got an image
	/*	if len(waifuImages) > 0 {
		waifuEmbed := WebhookEmbed{
			Title:       "💜 Daily Waifu",
			Description: "Here's your beautiful waifu for today!",
			Image:       &Image{URL: waifuImages[0].URL},
			Color:       0x9B59B6, // Purple color
		}
		payload.Embeds = append(payload.Embeds, waifuEmbed)
	}*/

	// Add catgirl embed if we got an image
	if len(catgirlImages) > 0 {
		catgirlEmbed := WebhookEmbed{
			Title:       "🐱 Daily Catgirl",
			Description: "And here's your adorable catgirl!",
			Image:       &Image{URL: fmt.Sprintf("https://nekos.moe/image/%s.jpg", catgirlImages[0].ID)},
			Color:       0xE91E63, // Pink color
		}
		payload.Embeds = append(payload.Embeds, catgirlEmbed)
	}

	// Send webhook
	log.Println("[WEBHOOK] Sending webhook payload...")
	return dw.sendWebhook(payload)
}

// sendWebhook sends the actual webhook request
func (dw *DailyWebhook) sendWebhook(payload WebhookPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	log.Printf("[WEBHOOK] Creating HTTP request to: %s", dw.webhookURL)
	log.Printf("[WEBHOOK] Payload size: %d bytes", len(jsonData))

	req, err := http.NewRequest("POST", dw.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	log.Println("[WEBHOOK] Sending HTTP request...")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[WEBHOOK] Webhook response status: %d", resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status code %d", resp.StatusCode)
	}

	dw.mutex.Lock()
	dw.lastSent = time.Now()
	dw.mutex.Unlock()

	log.Println("[WEBHOOK] Daily webhook sent successfully!")
	return nil
}

// GetLastSent returns the last time a daily webhook was sent
func (dw *DailyWebhook) GetLastSent() time.Time {
	dw.mutex.RLock()
	defer dw.mutex.RUnlock()
	return dw.lastSent
}

// isValidDiscordWebhookURL checks if the URL appears to be a valid Discord webhook URL
func isValidDiscordWebhookURL(url string) bool {
	// Discord webhook URLs should match this pattern:
	// https://discord.com/api/webhooks/{webhook.id}/{webhook.token}
	// or https://discordapp.com/api/webhooks/{webhook.id}/{webhook.token}
	pattern := `^https://(?:discord\.com|discordapp\.com)/api/webhooks/\d+/[a-zA-Z0-9_-]+$`
	matched, err := regexp.MatchString(pattern, url)
	if err != nil {
		log.Printf("[WEBHOOK] Error validating webhook URL: %v", err)
		return false
	}
	return matched
}
