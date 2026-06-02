package discord

import "context"

type Client interface {
	Start(ctx context.Context) error
	Close(ctx context.Context) error
}

type Interaction struct {
	GuildID   string
	ChannelID string
	UserID    string
	Name      string
	Text      string
}
