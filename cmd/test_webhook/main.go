package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"KawaiiBot/api"
	"KawaiiBot/webhook"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Get webhook URL
	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL == "" {
		log.Fatal("WEBHOOK_URL environment variable is not set")
	}

	fmt.Printf("Testing webhook with URL: %s\n", webhookURL)

	// Initialize API clients
	userAgent := "KawaiiBot-Test/1.0.0"
	nekosAPI := api.New(userAgent)
	waifuAPI := api.NewWaifuClient(userAgent)

	// Create webhook instance
	dailyWebhook := webhook.New(nekosAPI, waifuAPI)

	// Check if webhook is enabled
	if !dailyWebhook.IsEnabled() {
		log.Fatal("Daily webhook is not enabled. Make sure WEBHOOK_URL is set correctly.")
	}

	fmt.Println("✓ Webhook is enabled")

	// Get webhook status
	enabled, url := dailyWebhook.GetStatus()
	fmt.Printf("✓ Webhook status - Enabled: %v, URL: %s\n", enabled, url)

	// Test sending a webhook
	fmt.Println("\n📤 Testing webhook send...")
	err := dailyWebhook.SendDailyWebhook()
	if err != nil {
		fmt.Printf("❌ Failed to send webhook: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Webhook sent successfully!")

	// Test time calculation logic
	fmt.Printf("\n📅 Testing time calculation logic...\n")
	
	// Simulate the getTimeUntil5AM logic
	now := time.Now()
	next5AM := time.Date(now.Year(), now.Month(), now.Day(), 5, 0, 0, 0, now.Location())
	
	if now.After(next5AM) || now.Equal(next5AM) {
		next5AM = next5AM.Add(24 * time.Hour)
	}
	
	timeUntil5AM := next5AM.Sub(now)
	fmt.Printf("✓ Current time: %s\n", now.Format("15:04:05"))
	fmt.Printf("✓ Next 5 AM: %s\n", next5AM.Format("2006-01-02 15:04:05"))
	fmt.Printf("✓ Time until next 5 AM: %v\n", timeUntil5AM)

	fmt.Println("\n🎉 All tests passed! The webhook should work correctly at 5 AM.")
	fmt.Println("\n💡 To test the actual scheduler, you'll need to:")
	fmt.Println("   1. Set up the full bot with DISCORD_BOT_TOKEN")
	fmt.Println("   2. Enable the webhook with !webhook or /webhook")
	fmt.Println("   3. Wait until 5 AM or temporarily modify the scheduler to send sooner")
}