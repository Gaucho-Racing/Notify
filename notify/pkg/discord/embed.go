package discord

import (
	"fmt"
	"net/url"
	"time"
	"unicode/utf8"
)

// Limits from Discord's embed documentation. Checking them up front turns a
// guaranteed 400 on every delivery into one clear error for the sender.
const (
	MaxContentLength    = 2000
	maxEmbeds           = 10
	maxEmbedTotalChars  = 6000
	maxEmbedTitleChars  = 256
	maxEmbedDescription = 4096
	maxEmbedFields      = 25
	maxEmbedFieldName   = 256
	maxEmbedFieldValue  = 1024
	maxEmbedFooterText  = 2048
	maxEmbedAuthorName  = 256
	maxEmbedColor       = 0xFFFFFF
)

func ValidateEmbeds(embeds []Embed) error {
	if len(embeds) > maxEmbeds {
		return fmt.Errorf("at most %d embeds are allowed", maxEmbeds)
	}
	total := 0
	for i, embed := range embeds {
		prefix := fmt.Sprintf("embeds[%d]", i)
		count := func(field string, value string, limit int) error {
			n := utf8.RuneCountInString(value)
			if n > limit {
				return fmt.Errorf("%s.%s must be at most %d characters", prefix, field, limit)
			}
			total += n
			return nil
		}
		if err := count("title", embed.Title, maxEmbedTitleChars); err != nil {
			return err
		}
		if err := count("description", embed.Description, maxEmbedDescription); err != nil {
			return err
		}
		if embed.Color < 0 || embed.Color > maxEmbedColor {
			return fmt.Errorf("%s.color must be between 0 and 0xFFFFFF", prefix)
		}
		if embed.Timestamp != "" {
			if _, err := time.Parse(time.RFC3339, embed.Timestamp); err != nil {
				return fmt.Errorf("%s.timestamp must be an RFC 3339 time", prefix)
			}
		}
		urls := map[string]string{"url": embed.URL}
		if embed.Author != nil {
			if embed.Author.Name == "" {
				return fmt.Errorf("%s.author.name is required", prefix)
			}
			if err := count("author.name", embed.Author.Name, maxEmbedAuthorName); err != nil {
				return err
			}
			urls["author.url"] = embed.Author.URL
			urls["author.icon_url"] = embed.Author.IconURL
		}
		if embed.Footer != nil {
			if embed.Footer.Text == "" {
				return fmt.Errorf("%s.footer.text is required", prefix)
			}
			if err := count("footer.text", embed.Footer.Text, maxEmbedFooterText); err != nil {
				return err
			}
			urls["footer.icon_url"] = embed.Footer.IconURL
		}
		if embed.Image != nil {
			urls["image.url"] = embed.Image.URL
		}
		if embed.Thumbnail != nil {
			urls["thumbnail.url"] = embed.Thumbnail.URL
		}
		for field, value := range urls {
			if value != "" && !isHTTPURL(value) {
				return fmt.Errorf("%s.%s must be an absolute http(s) URL", prefix, field)
			}
		}
		if len(embed.Fields) > maxEmbedFields {
			return fmt.Errorf("%s may have at most %d fields", prefix, maxEmbedFields)
		}
		for j, field := range embed.Fields {
			if field.Name == "" || field.Value == "" {
				return fmt.Errorf("%s.fields[%d] needs both name and value", prefix, j)
			}
			if err := count(fmt.Sprintf("fields[%d].name", j), field.Name, maxEmbedFieldName); err != nil {
				return err
			}
			if err := count(fmt.Sprintf("fields[%d].value", j), field.Value, maxEmbedFieldValue); err != nil {
				return err
			}
		}
		if embed.Title == "" && embed.Description == "" && len(embed.Fields) == 0 && embed.Image == nil {
			return fmt.Errorf("%s is empty; set a title, description, fields, or image", prefix)
		}
	}
	if total > maxEmbedTotalChars {
		return fmt.Errorf("embeds may contain at most %d characters in total", maxEmbedTotalChars)
	}
	return nil
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}
