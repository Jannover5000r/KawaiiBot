// Package bot: used to do the whole bot part
package bot

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"KawaiiBot/api"
	"KawaiiBot/scheduler"
	"KawaiiBot/storage"
	"KawaiiBot/webhook"

	"github.com/bwmarrin/discordgo"
)

const (
	userAgent   = "KawaiiBot (kawaiibot, v1.0.0)"
	picturesDir = "pictures"
	maxFileAge  = 5 * time.Minute
	botStatus   = "Looking at anime girls"
)

// Bot represents the Discord bot
type Bot struct {
	session      *discordgo.Session
	nekosAPI     *api.Client
	waifuAPI     *api.WaifuClient
	fileMutex    sync.Mutex
	activeFiles  map[string]time.Time
	storage      *storage.Storage
	dailyWebhook *webhook.DailyWebhook
	scheduler    *scheduler.Scheduler
}

// New creates a new bot instance
func New(token string) (*Bot, error) {
	if err := os.MkdirAll(picturesDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create pictures directory: %w", err)
	}

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Set intents for guild messages, direct messages, and message content
	// MessageContent intent is now enabled in Discord Developer Portal
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent | discordgo.IntentsGuilds

	// Initialize storage
	storageInstance, err := storage.New("bot_settings.json")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize API clients
	nekosAPI := api.New(userAgent)
	waifuAPI := api.NewWaifuClient(userAgent)

	// Initialize webhook and scheduler
	dailyWebhook := webhook.New(nekosAPI, waifuAPI)
	schedulerInstance := scheduler.New(dailyWebhook)

	// Sync webhook enabled state with storage
	dailyWebhook.SetEnabled(storageInstance.GetDailyWebhookEnabled())

	bot := &Bot{
		session:      dg,
		nekosAPI:     nekosAPI,
		waifuAPI:     waifuAPI,
		activeFiles:  make(map[string]time.Time),
		storage:      storageInstance,
		dailyWebhook: dailyWebhook,
		scheduler:    schedulerInstance,
	}

	// Register handlers
	dg.AddHandler(bot.readyHandler)
	dg.AddHandler(bot.interactionHandler)
	dg.AddHandler(bot.messageHandler)

	return bot, nil
}

// Start opens the websocket connection and registers slash commands
func (b *Bot) Start(ctx context.Context) error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}

	// Register slash commands
	if err := b.registerCommands(); err != nil {
		return fmt.Errorf("failed to register commands: %w", err)
	}

	// Start cleanup routine
	go b.cleanupRoutine(ctx)

	// Start scheduler
	if err := b.scheduler.Start(ctx); err != nil {
		fmt.Printf("Warning: failed to start scheduler: %v\n", err)
	}

	return nil
}

// Stop closes the websocket connection and cleans up
func (b *Bot) Stop(ctx context.Context) error {
	// Stop scheduler
	if err := b.scheduler.Stop(); err != nil {
		fmt.Printf("Warning: failed to stop scheduler: %v\n", err)
	}

	// Unregister commands
	if err := b.unregisterCommands(); err != nil {
		fmt.Printf("Warning: failed to unregister commands: %v\n", err)
	}

	b.cleanupAllFiles()
	return b.session.Close()
}

// readyHandler is called when the bot is ready
func (b *Bot) readyHandler(s *discordgo.Session, event *discordgo.Ready) {
	fmt.Printf("Bot is ready! Logged in as %s#%s\n", event.User.Username, event.User.Discriminator)

	// Set custom status
	if err := s.UpdateListeningStatus(botStatus); err != nil {
		fmt.Printf("Failed to set status: %v\n", err)
	}
}

// sendImagesMessage sends images via regular message
func (b *Bot) sendImagesMessage(s *discordgo.Session, m *discordgo.MessageCreate, images []api.Image, message string) {
	files := make([]*discordgo.File, 0, len(images))

	for _, img := range images {
		// Generate unique filename
		filename := fmt.Sprintf("catgirl_%s_%d.jpg", img.ID, time.Now().Unix())
		filepath := filepath.Join(picturesDir, filename)

		// Download the image
		imageData, err := b.nekosAPI.DownloadImage(img.ID)
		if err != nil {
			continue
		}

		// Save to file
		if err := os.WriteFile(filepath, imageData, 0o644); err != nil {
			continue
		}

		// Track file for cleanup
		b.trackFile(filename)

		// Create file
		files = append(files, &discordgo.File{
			Name:        filename,
			ContentType: "image/jpg", // All images from nekos.moe are JPG
			Reader:      bytes.NewReader(imageData),
		})

		// Schedule file deletion
		go b.scheduleFileDeletion(filename, "")
	}

	// Send message with files only (no text content)
	_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files: files,
	})
	if err != nil {
		// Fallback to URLs
		var urls []string
		for _, img := range images {
			urls = append(urls, fmt.Sprintf("https://nekos.moe/image/%s.jpg", img.ID))
		}

		s.ChannelMessageSend(m.ChannelID, strings.Join(urls, "\n"))
	}
}

// sendWaifuImagesMessage sends waifu images via regular message
func (b *Bot) sendWaifuImagesMessage(s *discordgo.Session, m *discordgo.MessageCreate, images []api.WaifuImage, message string) {
	files := make([]*discordgo.File, 0, len(images))

	for _, img := range images {
		// Generate unique filename
		filename := fmt.Sprintf("waifu_%d_%d%s", img.ImageID, time.Now().Unix(), img.Extension)
		filepath := filepath.Join(picturesDir, filename)

		// Download the image
		imageData, err := b.waifuAPI.DownloadWaifuImage(img.URL)
		if err != nil {
			continue
		}

		// Save to file
		if err := os.WriteFile(filepath, imageData, 0o644); err != nil {
			continue
		}

		// Track file for cleanup
		b.trackFile(filename)

		// Determine content type based on extension
		contentType := "image/jpeg" // default
		switch img.Extension {
		case ".gif":
			contentType = "image/gif"
		case ".png":
			contentType = "image/png"
		case ".webp":
			contentType = "image/webp"
		}

		// Create file
		files = append(files, &discordgo.File{
			Name:        filename,
			ContentType: contentType,
			Reader:      bytes.NewReader(imageData),
		})

		// Schedule file deletion
		go b.scheduleFileDeletion(filename, "")
	}

	// Send message with files only (no text content)
	_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Files: files,
	})
	if err != nil {
		// Fallback to URLs
		var urls []string
		for _, img := range images {
			urls = append(urls, img.URL)
		}

		s.ChannelMessageSend(m.ChannelID, strings.Join(urls, "\n"))
	}
}

// handleCatgirlMessageCommand handles the !catgirl message command
func (b *Bot) handleCatgirlMessageCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Try to delete the user's command message (ignore errors as we might not have permission)
	go func() {
		err := s.ChannelMessageDelete(m.ChannelID, m.ID)
		if err != nil {
			// Silently ignore deletion errors (common in DMs or without manage messages permission)
		}
	}()

	// Parse command arguments
	args := strings.Fields(m.Content)

	var count int = 1     // Default count = 1     // Default count
	var nsfw string = "n" // Default to SFW

	// Parse count argument
	if len(args) > 1 {
		if parsedCount, err := strconv.Atoi(args[1]); err == nil && parsedCount >= 1 && parsedCount <= 10 {
			count = parsedCount
		}
	}

	// Parse NSFW argument
	if len(args) > 2 {
		if strings.ToLower(args[2]) == "y" || strings.ToLower(args[2]) == "yes" {
			nsfw = "y"
		}
	}

	// Determine rating
	rating := "safe"
	if nsfw == "y" {
		rating = "explicit"
	}

	// Show typing indicator
	s.ChannelTyping(m.ChannelID)

	// Fetch images
	images, err := b.nekosAPI.GetRandomImages(count, rating)
	if err != nil {
		content := fmt.Sprintf("Sorry, I couldn't fetch catgirl images: %v", err)
		s.ChannelMessageSend(m.ChannelID, content)
		return
	}

	if len(images) == 0 {
		content := "Sorry, no catgirl images found!"
		s.ChannelMessageSend(m.ChannelID, content)
		return
	}

	// Send images (no text content)
	b.sendImagesMessage(s, m, images, "")
}

// handleWaifuMessageCommand handles the !waifu message command
func (b *Bot) handleWaifuMessageCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Try to delete the user's command message (ignore errors as we might not have permission)
	go func() {
		err := s.ChannelMessageDelete(m.ChannelID, m.ID)
		if err != nil {
			// Silently ignore deletion errors (common in DMs or without manage messages permission)
		}
	}()

	// Parse command arguments
	args := strings.Fields(m.Content)

	var count int = 1     // Default count
	var nsfw string = "n" // Default to SFW
	var gif string = "n"  // Default to no GIFs

	// Track which parameters we've explicitly set
	countSet := false
	nsfwSet := false
	gifSet := false

	// Parse arguments - handle flexible positioning
	for i := 1; i < len(args); i++ {
		arg := strings.ToLower(args[i])

		// Check if this is a number (count)
		if parsedCount, err := strconv.Atoi(arg); err == nil && parsedCount >= 1 && parsedCount <= 10 {
			if !countSet {
				count = parsedCount
				countSet = true
			}
		} else if arg == "y" || arg == "yes" {
			// Set nsfw if not already set, otherwise set gif if not already set
			if !nsfwSet {
				nsfw = "y"
				nsfwSet = true
			} else if !gifSet {
				gif = "y"
				gifSet = true
			}
		} else if arg == "n" || arg == "no" {
			// Set nsfw if not already set, otherwise set gif if not already set
			if !nsfwSet {
				nsfw = "n"
				nsfwSet = true
			} else if !gifSet {
				gif = "n"
				gifSet = true
			}
		}
	}

	// Show typing indicator
	s.ChannelTyping(m.ChannelID)

	// Fetch images
	images, err := b.waifuAPI.GetWaifuImages(count, nsfw == "y", gif == "y")
	if err != nil {
		content := fmt.Sprintf("Sorry, I couldn't fetch waifu images: %v", err)
		s.ChannelMessageSend(m.ChannelID, content)
		return
	}

	if len(images) == 0 {
		content := "Sorry, no waifu images found!"
		s.ChannelMessageSend(m.ChannelID, content)
		return
	}

	// Send images (no text content)
	b.sendWaifuImagesMessage(s, m, images, "")
}

// handleHelpMessageCommand handles the !help message command
func (b *Bot) handleHelpMessageCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	helpText := "## 🌸 Kawaii Bot Help 🌸\n\n" +
		"*Your personal anime picture companion!*\n\n" +
		"### 📸 Available Commands\n\n" +
		"**🐱 Catgirl Commands**\n" +
		"├ `!catgirl [count] [nsfw]` - Message command\n" +
		"└ `/catgirl <count> [nsfw]` - Slash command\n" +
		"• **count**: 1-10 pictures (optional, defaults to 1)\n" +
		"• **nsfw**: `y/yes` or `n/no` (optional, defaults to no)\n\n" +
		"**💜 Waifu Commands**\n" +
		"├ `!waifu [count] [nsfw] [gif]` - Message command\n" +
		"└ `/waifu <count> [nsfw] [gif]` - Slash command\n" +
		"• **count**: 1-10 pictures (optional, defaults to 1)\n" +
		"• **nsfw**: `y/yes` or `n/no` (optional, defaults to no)\n" +
		"• **gif**: `y/yes` or `n/no` (optional, defaults to no)\n\n" +
		"**📅 Daily Webhook**\n" +
		"├ `!webhook` - Toggle daily webhook (message command)\n" +
		"└ `/webhook` - Toggle daily webhook (slash command)\n" +
		"• Sends 1 waifu + 1 catgirl picture daily at 5 AM\n" +
		"• Requires `WEBHOOK_URL` environment variable\n\n" +
		"### 💡 Tips\n" +
		"• Arguments can be in any order!\n" +
		"• Examples: `!waifu y`, `!waifu 5 y`, `!waifu y 3 n`\n" +
		"• Your command message will be automatically deleted\n\n" +
		"*Powered by Nekos.moe API & Waifu.im* 💕"

	s.ChannelMessageSend(m.ChannelID, helpText)
}

// handleWebhookMessageCommand handles the !webhook message command
func (b *Bot) handleWebhookMessageCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Check if webhook URL is configured
	_, url := b.dailyWebhook.GetStatus()
	if url == "" {
		s.ChannelMessageSend(m.ChannelID, "❌ Daily webhook is not configured. Please set the `WEBHOOK_URL` environment variable.")
		return
	}

	// Toggle the webhook status
	newState, err := b.storage.ToggleDailyWebhookEnabled()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ Failed to toggle webhook: %v", err))
		return
	}

	// Update the webhook enabled state
	b.dailyWebhook.SetEnabled(newState)

	// Create response message
	status := "disabled"
	emoji := "🔴"
	if newState {
		status = "enabled"
		emoji = "🟢"
	}

	response := fmt.Sprintf("%s Daily webhook is now **%s**!\n\n📅 **Schedule**: Every day at 6\n🌸 **Content**: 1 waifu + 1 catgirl picture\n🔗 **Webhook URL**: `%s`", emoji, status, url)
	s.ChannelMessageSend(m.ChannelID, response)
}

// messageHandler handles regular message commands
func (b *Bot) messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Message Content intent is now enabled, so prefix commands will work

	// Ignore messages from bots
	if m.Author.Bot {
		return
	}

	// Check for prefix commands
	if strings.HasPrefix(m.Content, "!catgirl") {
		b.handleCatgirlMessageCommand(s, m)
	} else if strings.HasPrefix(m.Content, "!waifu") {
		b.handleWaifuMessageCommand(s, m)
	} else if strings.HasPrefix(m.Content, "!help") {
		b.handleHelpMessageCommand(s, m)
	} else if strings.HasPrefix(m.Content, "!webhook") {
		b.handleWebhookMessageCommand(s, m)
	}
}

// registerCommands registers slash commands
func (b *Bot) registerCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "catgirl",
			Description: "Get adorable catgirl pictures 🐱",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "count",
					Description: "Number of pictures (1-10)",
					Required:    false,
					MinValue:    &[]float64{1}[0],
					MaxValue:    10,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "nsfw",
					Description: "Include NSFW content? (y=yes/n=no, defaults to no)",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Yes",
							Value: "y",
						},
						{
							Name:  "No",
							Value: "n",
						},
					},
				},
			},
		},
		{
			Name:        "waifu",
			Description: "Get beautiful waifu pictures 💜",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "count",
					Description: "Number of pictures (1-10)",
					Required:    false,
					MinValue:    &[]float64{1}[0],
					MaxValue:    10,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "nsfw",
					Description: "Include NSFW content? (y=yes/n=no, defaults to no)",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Yes",
							Value: "y",
						},
						{
							Name:  "No",
							Value: "n",
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "gif",
					Description: "Include GIFs? (y=yes/n=no, defaults to no)",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Yes",
							Value: "y",
						},
						{
							Name:  "No",
							Value: "n",
						},
					},
				},
			},
		},
		{
			Name:        "help",
			Description: "Show help information about the bot",
		},
		{
			Name:        "webhook",
			Description: "Toggle daily webhook for waifu/catgirl pictures",
		},
		{
			Name:        "forceWebhook",
			Description: "force send a WebHook for testing",
		},
	}

	// Register commands globally
	for _, cmd := range commands {
		_, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, "", cmd)
		if err != nil {
			return fmt.Errorf("failed to create command %s: %w", cmd.Name, err)
		}
	}

	return nil
}

// unregisterCommands removes slash commands
func (b *Bot) unregisterCommands() error {
	commands, err := b.session.ApplicationCommands(b.session.State.User.ID, "")
	if err != nil {
		return fmt.Errorf("failed to get commands: %w", err)
	}

	for _, cmd := range commands {
		if err := b.session.ApplicationCommandDelete(b.session.State.User.ID, "", cmd.ID); err != nil {
			fmt.Printf("Warning: failed to delete command %s: %v\n", cmd.Name, err)
		}
	}

	return nil
}

// interactionHandler handles slash command interactions
func (b *Bot) interactionHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()

	switch data.Name {
	case "catgirl":
		b.handleCatgirlSlashCommand(s, i, data)
	case "waifu":
		b.handleWaifuSlashCommand(s, i, data)
	case "help":
		b.handleHelpSlashCommand(s, i)
	case "webhook":
		b.handleWebhookSlashCommand(s, i)
	case "forceWebhook":
		b.forceWebHookSlashCommand(s, i)
	}
}

// handleCatgirlSlashCommand handles the /catgirl slash command
func (b *Bot) handleCatgirlSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	// Defer response to avoid timeout
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		fmt.Printf("Failed to defer interaction: %v\n", err)
		return
	}

	// Get options
	var count int
	var nsfw string = "n" // Default to SFW

	for _, option := range data.Options {
		switch option.Name {
		case "count":
			count = int(option.IntValue())
		case "nsfw":
			nsfw = strings.ToLower(strings.TrimSpace(option.StringValue()))
		}
	}

	// Validate and default NSFW parameter
	if nsfw != "y" && nsfw != "n" {
		nsfw = "n" // Default to SFW for any invalid input
	}

	// Determine rating
	rating := "safe"
	if nsfw == "y" {
		rating = "explicit"
	}

	// Show typing indicator
	s.ChannelTyping(i.ChannelID)

	// Fetch images
	images, err := b.nekosAPI.GetRandomImages(count, rating)
	if err != nil {
		content := fmt.Sprintf("Sorry, I couldn't fetch catgirl images: %v", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}

	if len(images) == 0 {
		content := "Sorry, no catgirl images found!"
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}

	// Send images (no text content)
	b.sendImagesInteraction(s, i, images, "")
}

// handleWaifuSlashCommand handles the /waifu slash command
func (b *Bot) handleWaifuSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	// Defer response to avoid timeout
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		fmt.Printf("Failed to defer interaction: %v\n", err)
		return
	}

	// Get options
	var count int
	var nsfw string = "n" // Default to SFW
	var gif string = "n"  // Default to no GIFs

	for _, option := range data.Options {
		switch option.Name {
		case "count":
			count = int(option.IntValue())
		case "nsfw":
			nsfw = strings.ToLower(strings.TrimSpace(option.StringValue()))
		case "gif":
			gif = strings.ToLower(strings.TrimSpace(option.StringValue()))
		}
	}

	// Validate and default parameters
	if nsfw != "y" && nsfw != "n" {
		nsfw = "n" // Default to SFW for any invalid input
	}
	if gif != "y" && gif != "n" {
		gif = "n" // Default to no GIFs for any invalid input
	}

	// Show typing indicator
	s.ChannelTyping(i.ChannelID)

	// Fetch images
	images, err := b.waifuAPI.GetWaifuImages(count, nsfw == "y", gif == "y")
	if err != nil {
		content := fmt.Sprintf("Sorry, I couldn't fetch waifu images: %v", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}

	if len(images) == 0 {
		content := "Sorry, no waifu images found!"
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return
	}

	// Send images (no text content)
	b.sendWaifuImagesInteraction(s, i, images, "")
}

// sendImagesInteraction sends images via interaction webhook
func (b *Bot) sendImagesInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, images []api.Image, message string) {
	files := make([]*discordgo.File, 0, len(images))

	for _, img := range images {
		// Generate unique filename
		filename := fmt.Sprintf("catgirl_%s_%d.jpg", img.ID, time.Now().Unix())
		filepath := filepath.Join(picturesDir, filename)

		// Download the image
		imageData, err := b.nekosAPI.DownloadImage(img.ID)
		if err != nil {
			continue
		}

		// Save to file
		if err := os.WriteFile(filepath, imageData, 0o644); err != nil {
			continue
		}

		// Track file for cleanup
		b.trackFile(filename)

		// Create file
		files = append(files, &discordgo.File{
			Name:        filename,
			ContentType: "image/jpg", // All images from nekos.moe are JPG
			Reader:      bytes.NewReader(imageData),
		})

		// Schedule file deletion
		go b.scheduleFileDeletion(filename, "")
	}

	// Send follow-up message with files only (no text content)
	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Files: files,
	})
	if err != nil {
		// Fallback to URLs (no text content)
		var urls []string
		for _, img := range images {
			urls = append(urls, fmt.Sprintf("https://nekos.moe/image/%s.jpg", img.ID))
		}

		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: strings.Join(urls, "\n"),
		})
	}
}

// sendWaifuImagesInteraction sends waifu images via interaction webhook
func (b *Bot) sendWaifuImagesInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, images []api.WaifuImage, message string) {
	files := make([]*discordgo.File, 0, len(images))

	for _, img := range images {
		// Generate unique filename
		filename := fmt.Sprintf("waifu_%d_%d%s", img.ImageID, time.Now().Unix(), img.Extension)
		filepath := filepath.Join(picturesDir, filename)

		// Download the image
		imageData, err := b.waifuAPI.DownloadWaifuImage(img.URL)
		if err != nil {
			continue
		}

		// Save to file
		if err := os.WriteFile(filepath, imageData, 0o644); err != nil {
			continue
		}

		// Track file for cleanup
		b.trackFile(filename)

		// Determine content type based on extension
		contentType := "image/jpeg" // default
		switch img.Extension {
		case ".gif":
			contentType = "image/gif"
		case ".png":
			contentType = "image/png"
		case ".webp":
			contentType = "image/webp"
		}

		// Create file
		files = append(files, &discordgo.File{
			Name:        filename,
			ContentType: contentType,
			Reader:      bytes.NewReader(imageData),
		})

		// Schedule file deletion
		go b.scheduleFileDeletion(filename, "")
	}

	// Send follow-up message with files only (no text content)
	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Files: files,
	})
	if err != nil {
		// Fallback to URLs (no text content)
		var urls []string
		for _, img := range images {
			urls = append(urls, img.URL)
		}

		s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: strings.Join(urls, "\n"),
		})
	}
}

// handleHelpSlashCommand handles the /help slash command
func (b *Bot) handleHelpSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	helpText := "## 🌸 Kawaii Bot Help 🌸\n\n" +
		"*Your personal anime picture companion!*\n\n" +
		"### 📸 Available Commands\n\n" +
		"**🐱 Catgirl Command**\n" +
		"`/catgirl <count> [nsfw]` - Get catgirl pictures\n" +
		"• **count**: 1-10 pictures (required)\n" +
		"• **nsfw**: `y/yes` or `n/no` (optional, defaults to no)\n\n" +
		"**💜 Waifu Command**\n" +
		"`/waifu <count> [nsfw] [gif]` - Get waifu pictures\n" +
		"• **count**: 1-10 pictures (required)\n" +
		"• **nsfw**: `y/yes` or `n/no` (optional, defaults to no)\n" +
		"• **gif**: `y/yes` or `n/no` (optional, defaults to no)\n\n" +
		"**📅 Daily Webhook**\n" +
		"`/webhook` - Toggle daily webhook\n" +
		"• Sends 1 waifu + 1 catgirl picture daily at 6 AM\n" +
		"• Requires `WEBHOOK_URL` environment variable\n\n" +
		"*Powered by Nekos.moe API & Waifu.im* 💕"

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: helpText,
		},
	})
}

func (b *Bot) forceWebHookSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	b.scheduler.ForceSend()
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "force sending Webhook",
		},
	})
}

// File management methods
func (b *Bot) trackFile(filename string) {
	b.fileMutex.Lock()
	defer b.fileMutex.Unlock()
	b.activeFiles[filename] = time.Now()
}

func (b *Bot) scheduleFileDeletion(filename string, messageID string) {
	time.Sleep(2 * time.Second)
	b.deleteFile(filename)
}

func (b *Bot) deleteFile(filename string) {
	b.fileMutex.Lock()
	defer b.fileMutex.Unlock()

	filepath := filepath.Join(picturesDir, filename)
	if err := os.Remove(filepath); err != nil {
		fmt.Printf("Warning: failed to delete file %s: %v\n", filename, err)
	}

	delete(b.activeFiles, filename)
}

func (b *Bot) cleanupAllFiles() {
	b.fileMutex.Lock()
	files := make([]string, 0, len(b.activeFiles))
	for filename := range b.activeFiles {
		files = append(files, filename)
	}
	b.fileMutex.Unlock()

	for _, filename := range files {
		b.deleteFile(filename)
	}
}

func (b *Bot) cleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.cleanupOldFiles()
		}
	}
}

// handleWebhookSlashCommand handles the /webhook slash command
func (b *Bot) handleWebhookSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Check if webhook URL is configured
	_, url := b.dailyWebhook.GetStatus()
	if url == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Daily webhook is not configured. Please set the `WEBHOOK_URL` environment variable.",
			},
		})
		return
	}

	// Toggle the webhook status
	newState, err := b.storage.ToggleDailyWebhookEnabled()
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("❌ Failed to toggle webhook: %v", err),
			},
		})
		return
	}

	// Update the webhook enabled state
	b.dailyWebhook.SetEnabled(newState)

	// Create response message
	status := "disabled"
	emoji := "🔴"
	if newState {
		status = "enabled"
		emoji = "🟢"
	}

	response := fmt.Sprintf("%s Daily webhook is now **%s**!\n\n📅 **Schedule**: Every day at midnight\n🌸 **Content**: 1 waifu + 1 catgirl picture\n🔗 **Webhook URL**: `%s`", emoji, status, url)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
		},
	})
}

func (b *Bot) cleanupOldFiles() {
	b.fileMutex.Lock()
	defer b.fileMutex.Unlock()

	now := time.Now()
	for filename, createdTime := range b.activeFiles {
		if now.Sub(createdTime) > maxFileAge {
			go b.deleteFile(filename)
		}
	}
}
