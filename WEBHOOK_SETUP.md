# Daily Webhook Setup Guide

## Overview
The KawaiiBot can send daily webhooks with waifu and catgirl pictures at 5 AM every day.

## Configuration

### 1. Set up a Discord Webhook
1. Go to your Discord server
2. Right-click on the channel where you want to receive daily pictures
3. Select "Edit Channel" → "Integrations" → "Webhooks"
4. Click "New Webhook"
5. Give it a name (e.g., "KawaiiBot Daily")
6. Copy the webhook URL

### 2. Configure Environment Variables
Add the webhook URL to your `.env` file:
```
WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN
```

### 3. Enable the Daily Webhook
Use either command:
- `!webhook` (message command)
- `/webhook` (slash command)

The bot will respond with the current status and confirm the webhook is enabled.

## Features

### Schedule
- **Time**: 5:00 AM daily (server time)
- **Content**: 1 waifu picture + 1 catgirl picture
- **Format**: Rich embed with images

### Error Handling
- Automatic retry with exponential backoff (up to 3 attempts)
- Detailed logging for troubleshooting
- Webhook URL validation

## Testing

### Manual Test
Run the test script to verify your webhook is working:
```bash
go run test_webhook.go
```

### Force Send Test
You can test the webhook immediately by temporarily modifying the scheduler code to send right away, or wait until 5 AM for the scheduled send.

## Troubleshooting

### Common Issues

1. **Webhook not sending**
   - Check that `WEBHOOK_URL` is set correctly in `.env`
   - Verify the webhook is enabled with `!webhook` or `/webhook`
   - Check bot logs for error messages
   - Ensure the webhook URL is valid (should match Discord's format)

2. **Wrong time**
   - The bot uses server time (where it's hosted)
   - Make sure your server time zone is correct
   - The webhook sends at 5:00 AM server time

3. **Images not loading**
   - Check internet connectivity
   - Verify the image APIs (nekos.moe and waifu.im) are accessible
   - Check for rate limiting

### Debug Logging
The bot now includes enhanced logging with prefixes:
- `[SCHEDULER]` - Scheduler-related logs
- `[WEBHOOK]` - Webhook-specific logs

Check your logs for these prefixes to see detailed information about webhook attempts.

## Webhook URL Validation
The bot validates webhook URLs to ensure they match Discord's expected format:
- Must start with `https://discord.com/api/webhooks/` or `https://discordapp.com/api/webhooks/`
- Must have a numeric webhook ID
- Must have a valid webhook token

If your URL doesn't match this pattern, you'll see a warning in the logs, but the bot will still attempt to use it.