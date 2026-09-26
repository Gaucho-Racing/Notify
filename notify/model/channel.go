package model

type Channel string

const (
	ChannelEmail   Channel = "EMAIL"
	ChannelDiscord Channel = "DISCORD"
)

var SupportedChannels = []Channel{ChannelEmail, ChannelDiscord}

func (c Channel) Valid() bool {
	for _, supported := range SupportedChannels {
		if c == supported {
			return true
		}
	}
	return false
}
