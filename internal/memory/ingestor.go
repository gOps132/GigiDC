package memory

import (
	"context"
	"time"
)

type IngestStore interface {
	GuildPolicy(ctx context.Context, guildID string) (Policy, error)
	ChannelPolicy(ctx context.Context, guildID string, channelID string) (ChannelPolicy, bool, error)
	RecordMessage(ctx context.Context, record MessageRecord) error
	DeleteMessage(ctx context.Context, guildID string, messageID string, deletedAt time.Time) error
}

type LiveIngestor struct {
	store IngestStore
	queue chan MessageEvent
	now   func() time.Time
}

func NewLiveIngestor(store IngestStore, queueSize int) *LiveIngestor {
	if queueSize <= 0 {
		queueSize = 256
	}
	ingestor := &LiveIngestor{
		store: store,
		queue: make(chan MessageEvent, queueSize),
		now:   time.Now,
	}
	go ingestor.run()
	return ingestor
}

func (i *LiveIngestor) TryEnqueueMessage(event MessageEvent) bool {
	if i == nil || i.store == nil {
		return false
	}
	select {
	case i.queue <- event:
		return true
	default:
		return false
	}
}

func (i *LiveIngestor) run() {
	for event := range i.queue {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = i.ingest(ctx, event)
		cancel()
	}
}

func (i *LiveIngestor) ingest(ctx context.Context, event MessageEvent) error {
	if event.Deleted {
		if event.GuildID == "" || event.MessageID == "" {
			return nil
		}
		deletedAt := event.DeletedAt
		if deletedAt.IsZero() {
			deletedAt = i.currentTime()
		}
		return i.store.DeleteMessage(ctx, event.GuildID, event.MessageID, deletedAt)
	}
	event.Content = NormalizeText(event.Content)
	if event.GuildID == "" || event.ChannelID == "" || event.MessageID == "" || event.AuthorUserID == "" || event.Content == "" {
		return nil
	}
	channel, ok, err := i.store.ChannelPolicy(ctx, event.GuildID, event.ChannelID)
	if err != nil || !ok || channel.Mode == ModeOff {
		return err
	}
	policy, err := i.store.GuildPolicy(ctx, event.GuildID)
	if err != nil {
		return err
	}
	retentionDays := channel.RetentionDays
	if retentionDays == 0 {
		retentionDays = policy.DefaultRetentionDays
	}
	if retentionDays <= 0 {
		retentionDays = DefaultRetentionDays
	}
	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = i.currentTime()
	}
	normalizedText := ""
	if channel.Mode == ModeFull {
		normalizedText = event.Content
	}
	return i.store.RecordMessage(ctx, MessageRecord{
		MessageID:      event.MessageID,
		GuildID:        event.GuildID,
		ChannelID:      event.ChannelID,
		AuthorUserID:   event.AuthorUserID,
		NormalizedText: normalizedText,
		ContentHash:    HashText(event.Content),
		CreatedAt:      createdAt,
		RetentionUntil: createdAt.AddDate(0, 0, retentionDays),
	})
}

func (i *LiveIngestor) currentTime() time.Time {
	if i.now == nil {
		return time.Now()
	}
	return i.now()
}
