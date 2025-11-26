# Discord Bot for sending anime pictures, self hosted

## Features

- **Interactive Commands**: Get catgirl and waifu pictures on demand
- **Daily Webhook**: Automatically sends motivational waifu/catgirl pictures daily at 5 AM
- **Flexible Options**: Choose between SFW/NSFW content, GIFs, and picture count
- **Multiple Interfaces**: Both message commands (`!command`) and slash commands (`/command`)

## Setup

1. Clone the repository
2. Create a `.env` file based on `.env.example`
3. Add your Discord bot token to the `.env` file
4. (Optional) Add a webhook URL for daily picture delivery
5. Run `go build` and start the bot

## Commands

Get the full command list by typing `!help` or `/help` in DMs.

### Picture Commands
- **Catgirl**: `!catgirl [count] [nsfw]` or `/catgirl <count> [nsfw]`
- **Waifu**: `!waifu [count] [nsfw] [gif]` or `/waifu <count> [nsfw] [gif]`

### Daily Webhook
- **Toggle**: `!webhook` or `/webhook`
- Sends 1 waifu + 1 catgirl picture daily at midnight
- Requires `WEBHOOK_URL` environment variable to be set

## APIs Used

- [Catgirl Pictures](https://docs.nekos.moe/) [Website](https://nekos.moe/)
- [Waifu Pictures](https://docs.waifu.im/) [Website](https://www.waifu.im/)
