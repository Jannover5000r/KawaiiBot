package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"KawaiiBot/bot"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Get bot token
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Fatal("Please set DISCORD_BOT_TOKEN environment variable")
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create bot instance
	discordBot, err := bot.New(token)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	// Start bot
	if err := discordBot.Start(ctx); err != nil {
		log.Fatalf("Error starting bot: %v", err)
	}

	log.Println("Bot is now running. Press CTRL+C to exit.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := discordBot.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}
