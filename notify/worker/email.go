package worker

import (
	"bytes"
	"context"
	"html/template"
	"strings"
	"time"

	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/pkg/resend"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
)

type emailSender struct{}

func (emailSender) Provider() string {
	return "resend"
}

func (emailSender) Destination(contact sentinel.Contact) (string, bool) {
	return contact.Email, contact.Email != ""
}

func (emailSender) MaskDestination(destination string) string {
	local, domain, found := strings.Cut(destination, "@")
	if !found || local == "" {
		return "***"
	}
	return string([]rune(local)[0]) + "***@" + domain
}

func (emailSender) Send(ctx context.Context, outbound Outbound, destination string) (string, error) {
	html := outbound.Message.HTML
	if html == "" {
		rendered, err := renderEmailHTML(outbound)
		if err != nil {
			return "", err
		}
		html = rendered
	}
	return resend.Send(ctx, resend.Email{
		From:    config.EmailFrom,
		To:      []string{destination},
		Subject: outbound.Message.Subject,
		Text:    emailText(outbound),
		HTML:    html,
	}, outbound.DeliveryID)
}

func emailText(outbound Outbound) string {
	text := outbound.Message.Body
	if outbound.Message.ActionURL != "" {
		text += "\n\n" + actionLabel(outbound.Message.ActionLabel) + ": " + outbound.Message.ActionURL
	}
	return text
}

func actionLabel(label string) string {
	if label == "" {
		return "Open"
	}
	return label
}

var emailTemplate = template.Must(template.New("email").Parse(`<!DOCTYPE html>
<html>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;padding:32px 16px;">
<tr><td align="center">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#ffffff;border-radius:8px;padding:32px;">
<tr><td style="font-size:20px;font-weight:600;color:#18181b;padding-bottom:16px;">{{.Subject}}</td></tr>
{{range .Paragraphs}}<tr><td style="font-size:15px;line-height:1.6;color:#3f3f46;padding-bottom:12px;">{{.}}</td></tr>
{{end}}{{if .ActionURL}}<tr><td style="padding-top:12px;"><a href="{{.ActionURL}}" style="display:inline-block;background:#18181b;color:#ffffff;text-decoration:none;font-size:14px;font-weight:600;padding:10px 18px;border-radius:6px;">{{.ActionLabel}}</a></td></tr>
{{end}}</table>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;">
<tr><td style="font-size:12px;color:#a1a1aa;padding-top:16px;text-align:center;">Sent by {{.Sender}} via Gaucho Racing Notify &middot; {{.Year}}</td></tr>
</table>
</td></tr>
</table>
</body>
</html>`))

func renderEmailHTML(outbound Outbound) (string, error) {
	paragraphs := []string{}
	for _, paragraph := range strings.Split(strings.ReplaceAll(outbound.Message.Body, "\r\n", "\n"), "\n\n") {
		if paragraph = strings.TrimSpace(paragraph); paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
	}
	sender := outbound.ApplicationName
	if sender == "" {
		sender = "Gaucho Racing"
	}

	var buf bytes.Buffer
	if err := emailTemplate.Execute(&buf, map[string]interface{}{
		"Subject":     outbound.Message.Subject,
		"Paragraphs":  paragraphs,
		"ActionURL":   outbound.Message.ActionURL,
		"ActionLabel": actionLabel(outbound.Message.ActionLabel),
		"Sender":      sender,
		"Year":        time.Now().Year(),
	}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
