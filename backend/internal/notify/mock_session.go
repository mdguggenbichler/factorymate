package notify

import (
	"github.com/bwmarrin/discordgo"
)

// MockChannelSend records a channel message send call.
type MockChannelSend struct {
	ChannelID string
	Message   *discordgo.MessageSend
}

// MockDiscordSession is a test double for DiscordSession.
type MockDiscordSession struct {
	ChannelCalls []MockChannelSend
	DMUserIDs    []string
	DMChannelID  string
}

// NewMockDiscordSession returns a mock with a default DM channel ID.
func NewMockDiscordSession() *MockDiscordSession {
	return &MockDiscordSession{DMChannelID: "dm-channel-1"}
}

func (m *MockDiscordSession) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	m.ChannelCalls = append(m.ChannelCalls, MockChannelSend{ChannelID: channelID, Message: data})
	return &discordgo.Message{ID: "msg-1"}, nil
}

func (m *MockDiscordSession) UserChannelCreate(recipientID string, _ ...discordgo.RequestOption) (*discordgo.Channel, error) {
	m.DMUserIDs = append(m.DMUserIDs, recipientID)
	return &discordgo.Channel{ID: m.DMChannelID}, nil
}
