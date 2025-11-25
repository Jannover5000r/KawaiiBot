// Package webhook handles daily webhook functionality for sending anime pictures
package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"KawaiiBot/api"
)

// DailyWebhook handles the daily webhook functionality
type DailyWebhook struct {
	webhookURL    string
	nekosAPI      *api.Client
	waifuAPI      *api.WaifuClient
	enabled       bool
	mutex         sync.RWMutex
	lastSent      time.Time
}

// New creates a new DailyWebhook instance
func New(nekosAPI *api.Client, waifuAPI *api.WaifuClient) *DailyWebhook {
	webhookURL := os.Getenv("WEBHOOK_URL")
	
	dw := &DailyWebhook{
		webhookURL: webhookURL,
		nekosAPI:   nekosAPI,
		waifuAPI:   waifuAPI,
		enabled:    webhookURL != "", // Enabled if URL is configured
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
}

// Toggle toggles the enabled status of the daily webhook
func (dw *DailyWebhook) Toggle() bool {
	dw.mutex.Lock()
	defer dw.mutex.Unlock()
	dw.enabled = !dw.enabled
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
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Image       *Image  `json:"image,omitempty"`
	Color       int     `json:"color,omitempty"`
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

	log.Println("Sending daily webhook...")

	// Fetch one waifu image
	waifuImages, err := dw.waifuAPI.GetWaifuImages(1, false, false)
	if err != nil {
		return fmt.Errorf("failed to fetch waifu image: %w", err)
	}

	// Fetch one catgirl image
	catgirlImages, err := dw.nekosAPI.GetRandomImages(1, "safe")
	if err != nil {
		return fmt.Errorf("failed to fetch catgirl image: %w", err)
	}

	// Create webhook payload
	payload := WebhookPayload{
		Content: "## 🌸 Your daily motivational waifu/catgirl 🌸\n*Starting your day with some kawaii energy!* 💕",
		Embeds:  []WebhookEmbed{},
	}

	// Add waifu embed if we got an image
	if len(waifuImages) > 0 {
		waifuEmbed := WebhookEmbed{
			Title:       "💜 Daily Waifu",
			Description: "Here's your beautiful waifu for today!",
			Image:       &Image{URL: waifuImages[0].URL},
			Color:       0x9B59B6, // Purple color
		}
		payload.Embeds = append(payload.Embeds, waifuEmbed)
	}

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
	return dw.sendWebhook(payload)
}

// sendWebhook sends the actual webhook request
func (dw *DailyWebhook) sendWebhook(payload WebhookPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequest("POST", dw.webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status code %d", resp.StatusCode)
	}

	dw.mutex.Lock()
	dw.lastSent = time.Now()
	dw.mutex.Unlock()

	log.Println("Daily webhook sent successfully!")
	return nil
}

// GetLastSent returns the last time a daily webhook was sent
func (dw *DailyWebhook) GetLastSent() time.Time {
	dw.mutex.RLock()
	defer dw.mutex.RUnlock()
	return dw.lastSent
}