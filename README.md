# Notify

<img align="right" width="159px" src="assets/notify.png" alt="notify-logo">

[![build](https://github.com/Gaucho-Racing/Notify/actions/workflows/build.yml/badge.svg)](https://github.com/Gaucho-Racing/Notify/actions/workflows/build.yml)
[![Release](https://img.shields.io/github/release/gaucho-racing/notify.svg?style=flat-square)](https://github.com/gaucho-racing/notify/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Notify is Gaucho Racing's notification service.
It gives every team application one API for reaching members by email, SMS, or Discord, so apps don't each need their own Resend keys, Twilio numbers, or bot tokens.

Notify integrates with [Sentinel](https://github.com/Gaucho-Racing/Sentinel) for authentication and recipient lookup.
Applications call the API with a Sentinel service account token, and an admin-managed policy decides which channels each application may use, whether it can message whole groups, and how many deliveries it can send per day.
Recipients are Sentinel users and groups, and Notify resolves each person's email address, phone number, or Discord account when it delivers.

Every message targets one channel:

- **Email** through Resend, as plain text or with caller-supplied HTML.
- **SMS** through Twilio, with delivery receipts so undelivered texts are marked failed.
- **Discord** through bot accounts, as direct messages or posts in server channels, with optional embeds and a temporary webhook for a custom name and avatar.

Deliveries go through a Postgres-backed queue with retries and backoff.
The web console lets admins manage senders, Twilio and Resend accounts, Discord bots, and application access, watch delivery activity, and send test messages from a playground.

## Getting Started

### Prerequisites

- Docker with Docker Compose
- A Sentinel OAuth client configured with `http://localhost:10310/auth/login` as a redirect URI
- A Sentinel service account token with `user:read`, `groups:read`, and `applications:read`, used to resolve recipients and list applications

### Local Development

1. Clone the repository and enter the project directory.

   ```sh
   git clone https://github.com/Gaucho-Racing/Notify.git
   cd Notify
   ```

2. Create the local environment file.

   ```sh
   cp example.env .env
   ```

3. Set `SENTINEL_CLIENT_ID`, `SENTINEL_CLIENT_SECRET`, `SENTINEL_SA_TOKEN`, and `ENCRYPTION_KEY` in `.env`.

4. Start Notify.

   ```sh
   docker compose up --build
   ```

5. Open [http://localhost:10310](http://localhost:10310).

The local stack runs the React frontend, Go API and delivery worker, Notify database, and Kerbecs gateway.
Database migrations are applied automatically when the API starts.

### Sending a Message

Applications send with `POST /api/messages`, authenticated with their Sentinel service account token.
The application needs a policy in Notify (the Applications page) that allows the channel.

```sh
curl -X POST https://notify.gauchoracing.com/api/messages \
  -H "Authorization: Bearer $NOTIFY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "DISCORD",
    "recipients": { "entity_ids": [], "group_ids": [], "discord_channel_ids": ["1253546219498766357"] },
    "discord": { "content": "Hello from Notify!" }
  }'
```

Each request carries exactly one content block matching `channel`:

| Channel | Block | Fields |
| --- | --- | --- |
| `EMAIL` | `email` | `sender_id`, `subject`, `body`, `html` |
| `SMS` | `sms` | `sender_id`, `body` |
| `DISCORD` | `discord` | `bot_id`, `content`, `embeds`, `webhook` |

Sender and bot IDs are optional and fall back to the channel's default.
`discord_channel_ids` is accepted only for Discord.

### Configuration

| Variable | Description |
| --- | --- |
| `SENTINEL_URL` | Base URL of the Sentinel deployment. |
| `SENTINEL_CLIENT_ID` | Sentinel OAuth client ID used by the API and web application. |
| `SENTINEL_CLIENT_SECRET` | Sentinel OAuth client secret used for authorization-code exchange. |
| `SENTINEL_REDIRECT_URI` | OAuth callback URL registered with Sentinel. |
| `SENTINEL_SA_TOKEN` | Sentinel service account token used to resolve users, groups, and contact details. |
| `ENCRYPTION_KEY` | Base64-encoded 32-byte key used to encrypt Discord bot tokens, Resend API keys, and Twilio secrets at rest. |
| `PUBLIC_API_URL` | Public URL of the API (for example `https://notify.gauchoracing.com/api`), used for Twilio delivery receipts. |
| `WORKER_CONCURRENCY` | Number of deliveries processed in parallel. Defaults to `8`. |
| `DELIVERY_MAX_ATTEMPTS` | Attempts before a delivery is marked failed. Defaults to `6`. |

Generate a production encryption key with:

```sh
openssl rand -base64 32
```

### Validation

```sh
cd web
npm ci
npm run lint
npm run build

cd ../notify
go build ./...
go vet ./...
```

### Releases

Run the release script from a clean, up-to-date `main` branch:

```sh
./scripts/release.sh 1.2.0
```

The script updates the application version, creates the release commit, and publishes a GitHub release.
The release workflows publish multi-architecture `notify-server` and `notify-web` images to GitHub Container Registry, then open an infrastructure pull request with the new image tags.

## Contributing

If you have a suggestion that would make this better, please fork the repo and create a pull request. You can also open an issue with the tag `enhancement`.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b gh-username/my-amazing-feature`)
3. Commit your Changes (`git commit -m 'Add my amazing feature'`)
4. Push to the Branch (`git push origin gh-username/my-amazing-feature`)
5. Open a Pull Request

## License

Distributed under the MIT License. See `LICENSE.txt` for more information.
