package discord

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
)

const (
	permissionAdministrator = 1 << 3
	permissionViewChannel   = 1 << 10
	permissionSendMessages  = 1 << 11
	permissionAll           = ^uint64(0)

	channelTypeText         = 0
	channelTypeCategory     = 4
	channelTypeAnnouncement = 5

	overwriteTypeRole   = 0
	overwriteTypeMember = 1
)

type Guild struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	IconURL  string         `json:"icon_url,omitempty"`
	Channels []GuildChannel `json:"channels"`
}

type GuildChannel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	// Announcement channels post like text channels but are followable from
	// other servers.
	Announcement bool `json:"announcement,omitempty"`
}

type partialGuild struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Owner       bool   `json:"owner"`
	Permissions string `json:"permissions"`
}

type channelObject struct {
	ID                   string      `json:"id"`
	Type                 int         `json:"type"`
	Name                 string      `json:"name"`
	Position             int         `json:"position"`
	ParentID             string      `json:"parent_id"`
	PermissionOverwrites []overwrite `json:"permission_overwrites"`
}

type overwrite struct {
	ID    string `json:"id"`
	Type  int    `json:"type"`
	Allow string `json:"allow"`
	Deny  string `json:"deny"`
}

// ListPostableChannels returns, per guild the bot is in, the text and
// announcement channels where it can both view and send messages, ordered as
// Discord shows them.
func ListPostableChannels(ctx context.Context, token string, botUserID string) ([]Guild, error) {
	var partials []partialGuild
	if err := call(ctx, token, http.MethodGet, "/users/@me/guilds", nil, &partials); err != nil {
		return nil, err
	}
	guilds := make([]Guild, 0, len(partials))
	for _, partial := range partials {
		var channels []channelObject
		if err := call(ctx, token, http.MethodGet, "/guilds/"+url.PathEscape(partial.ID)+"/channels", nil, &channels); err != nil {
			return nil, err
		}
		var member struct {
			Roles []string `json:"roles"`
		}
		if err := call(ctx, token, http.MethodGet, "/guilds/"+url.PathEscape(partial.ID)+"/members/"+url.PathEscape(botUserID), nil, &member); err != nil {
			return nil, err
		}
		guild := Guild{ID: partial.ID, Name: partial.Name}
		if partial.Icon != "" {
			guild.IconURL = "https://cdn.discordapp.com/icons/" + url.PathEscape(partial.ID) + "/" + url.PathEscape(partial.Icon) + ".png"
		}
		guild.Channels = postableChannels(partial, member.Roles, botUserID, channels)
		guilds = append(guilds, guild)
	}
	sort.Slice(guilds, func(i, j int) bool { return guilds[i].Name < guilds[j].Name })
	return guilds, nil
}

func postableChannels(guild partialGuild, roles []string, botUserID string, channels []channelObject) []GuildChannel {
	base := basePermissions(guild)
	categories := map[string]channelObject{}
	for _, channel := range channels {
		if channel.Type == channelTypeCategory {
			categories[channel.ID] = channel
		}
	}
	// Discord lists uncategorized channels first, then each category in
	// position order with its channels in position order.
	order := func(channel channelObject) (int, int) {
		if parent, ok := categories[channel.ParentID]; ok {
			return parent.Position + 1, channel.Position
		}
		return 0, channel.Position
	}
	eligible := []channelObject{}
	for _, channel := range channels {
		if channel.Type != channelTypeText && channel.Type != channelTypeAnnouncement {
			continue
		}
		permissions := channelPermissions(base, guild.ID, roles, botUserID, channel.PermissionOverwrites)
		if permissions&permissionViewChannel == 0 || permissions&permissionSendMessages == 0 {
			continue
		}
		eligible = append(eligible, channel)
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		gi, pi := order(eligible[i])
		gj, pj := order(eligible[j])
		if gi != gj {
			return gi < gj
		}
		return pi < pj
	})
	result := make([]GuildChannel, 0, len(eligible))
	for _, channel := range eligible {
		result = append(result, GuildChannel{
			ID:           channel.ID,
			Name:         channel.Name,
			Category:     categories[channel.ParentID].Name,
			Announcement: channel.Type == channelTypeAnnouncement,
		})
	}
	return result
}

// basePermissions uses the guild-level permissions Discord reports for the
// bot, which already combine @everyone and the bot's roles.
func basePermissions(guild partialGuild) uint64 {
	if guild.Owner {
		return permissionAll
	}
	permissions := parseBits(guild.Permissions)
	if permissions&permissionAdministrator != 0 {
		return permissionAll
	}
	return permissions
}

// channelPermissions applies a channel's overwrites in Discord's documented
// order: @everyone, then the bot's roles together, then the bot itself.
func channelPermissions(base uint64, guildID string, roles []string, botUserID string, overwrites []overwrite) uint64 {
	if base == permissionAll {
		return permissionAll
	}
	permissions := base
	for _, o := range overwrites {
		if o.Type == overwriteTypeRole && o.ID == guildID {
			permissions &^= parseBits(o.Deny)
			permissions |= parseBits(o.Allow)
		}
	}
	var allow, deny uint64
	for _, o := range overwrites {
		if o.Type == overwriteTypeRole && o.ID != guildID && slices.Contains(roles, o.ID) {
			allow |= parseBits(o.Allow)
			deny |= parseBits(o.Deny)
		}
	}
	permissions &^= deny
	permissions |= allow
	for _, o := range overwrites {
		if o.Type == overwriteTypeMember && o.ID == botUserID {
			permissions &^= parseBits(o.Deny)
			permissions |= parseBits(o.Allow)
		}
	}
	return permissions
}

func parseBits(value string) uint64 {
	bits, _ := strconv.ParseUint(value, 10, 64)
	return bits
}
