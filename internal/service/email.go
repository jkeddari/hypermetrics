package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/resend/resend-go/v3"
)

type EmailService struct {
	client     *resend.Client
	fromEmail  string
	audienceID string
	isDev      bool
	appURL     string
	appName    string
}

func NewEmailService(apiKey, fromEmail, audienceID, appURL, appName string, isDev bool) *EmailService {
	var client *resend.Client
	if apiKey != "" && !isDev {
		client = resend.NewClient(apiKey)
	}

	return &EmailService{
		client:     client,
		fromEmail:  fromEmail,
		audienceID: audienceID,
		isDev:      isDev,
		appURL:     appURL,
		appName:    appName,
	}
}

func (s *EmailService) SendForgotPasswordEmail(email, token, name string) error {
	signInURL := fmt.Sprintf("%s/auth/forgot-password/%s", s.appURL, token)
	subject, body := forgotPasswordEmailTemplate(signInURL, s.appName)

	if s.isDev {
		slog.Info("email sent (dev mode)", "type", "forgot_password", "to", email, "subject", subject, "url", signInURL)
		return nil
	}

	if s.client == nil {
		return fmt.Errorf("email service not configured (missing RESEND_API_KEY)")
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{email},
		ReplyTo: s.fromEmail,
		Subject: subject,
		Text:    body,
		Html:    textToHTML(body),
	}

	_, err := s.client.Emails.SendWithContext(context.Background(), params)
	if err == nil {
		slog.Info("email sent", "type", "forgot_password", "to", email)
	}
	return err
}

func (s *EmailService) SendMagicLinkEmail(email, token, name string) error {
	magicURL := fmt.Sprintf("%s/auth/magic-link/%s", s.appURL, token)
	subject, body := magicLinkEmailTemplate(magicURL, s.appName)

	if s.isDev {
		slog.Info("email sent (dev mode)", "type", "magic_link", "to", email, "subject", subject, "url", magicURL)
		return nil
	}

	if s.client == nil {
		return fmt.Errorf("email service not configured (missing RESEND_API_KEY)")
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{email},
		ReplyTo: s.fromEmail,
		Subject: subject,
		Text:    body,
		Html:    textToHTML(body),
	}

	_, err := s.client.Emails.SendWithContext(context.Background(), params)
	if err == nil {
		slog.Info("email sent", "type", "magic_link", "to", email)
	}
	return err
}

func (s *EmailService) SubscribeNewsletter(email string) error {
	if s.isDev {
		slog.Info("newsletter subscription (dev mode)", "email", email)
		return nil
	}

	if s.client == nil {
		return fmt.Errorf("email service not configured (missing RESEND_API_KEY)")
	}

	if s.audienceID == "" {
		// If no audience ID is configured, just log and return
		slog.Warn("newsletter subscription requested but no audience configured", "email", email)
		return nil
	}

	contact, err := s.client.Contacts.Create(&resend.CreateContactRequest{
		Email: email,
	})
	if err != nil {
		slog.Warn("newsletter subscription failed", "error", err, "email", email)
		// Ignore errors to prevent email enumeration
		// This includes duplicates, invalid emails, or API issues
		return nil
	}

	_, err = s.client.Contacts.Segments.Add(&resend.AddContactSegmentRequest{
		SegmentId: s.audienceID,
		ContactId: contact.Id,
	})
	if err != nil {
		slog.Warn("newsletter subscription failed", "error", err, "email", email)
		// Ignore errors to prevent email enumeration
		// This includes duplicates, invalid emails, or API issues
		return nil
	}

	slog.Info("newsletter subscription successful", "email", email)
	return nil
}

func (s *EmailService) SendWelcomeEmail(email, name string) error {
	dashboardURL := fmt.Sprintf("%s/app/dashboard", s.appURL)
	subject, body := welcomeEmailTemplate(name, dashboardURL, s.appName)

	if s.isDev {
		slog.Info("email sent (dev mode)", "type", "welcome", "to", email, "subject", subject, "url", dashboardURL)
		return nil
	}

	if s.client == nil {
		return fmt.Errorf("email service not configured (missing RESEND_API_KEY)")
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{email},
		ReplyTo: s.fromEmail,
		Subject: subject,
		Text:    body,
		Html:    textToHTML(body),
	}

	_, err := s.client.Emails.SendWithContext(context.Background(), params)
	if err == nil {
		slog.Info("email sent", "type", "welcome", "to", email)
	}
	return err
}

func (s *EmailService) SendEmailChangeVerification(newEmail, token, userName string) error {
	verifyURL := fmt.Sprintf("%s/auth/verify-email-change/%s", s.appURL, token)
	subject, body := emailChangeVerificationTemplate(userName, verifyURL, s.appName)

	if s.isDev {
		slog.Info("email sent (dev mode)", "type", "email_change_verification", "to", newEmail, "subject", subject, "url", verifyURL)
		return nil
	}

	if s.client == nil {
		return fmt.Errorf("email service not configured (missing RESEND_API_KEY)")
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{newEmail},
		ReplyTo: s.fromEmail,
		Subject: subject,
		Text:    body,
		Html:    textToHTML(body),
	}

	_, err := s.client.Emails.SendWithContext(context.Background(), params)
	if err == nil {
		slog.Info("email sent", "type", "email_change_verification", "to", newEmail)
	}
	return err
}

func (s *EmailService) SendEmailChangeNotification(oldEmail, newEmail, userName string) error {
	subject, body := emailChangeNotificationTemplate(userName, newEmail, s.appName)

	if s.isDev {
		slog.Info("email sent (dev mode)", "type", "email_change_notification", "to", oldEmail, "new_email", newEmail)
		return nil
	}

	if s.client == nil {
		return fmt.Errorf("email service not configured (missing RESEND_API_KEY)")
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{oldEmail},
		ReplyTo: s.fromEmail,
		Subject: subject,
		Text:    body,
		Html:    textToHTML(body),
	}

	_, err := s.client.Emails.SendWithContext(context.Background(), params)
	if err == nil {
		slog.Info("email sent", "type", "email_change_notification", "to", oldEmail)
	}
	return err
}

func (s *EmailService) SendAccountDeletedEmail(email, name string) error {
	subject, body := accountDeletedEmailTemplate(name, s.appName)

	if s.isDev {
		slog.Info("email sent (dev mode)", "type", "account_deleted", "to", email, "subject", subject)
		return nil
	}

	if s.client == nil {
		return fmt.Errorf("email service not configured (missing RESEND_API_KEY)")
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{email},
		ReplyTo: s.fromEmail,
		Subject: subject,
		Text:    body,
		Html:    textToHTML(body),
	}

	_, err := s.client.Emails.SendWithContext(context.Background(), params)
	if err == nil {
		slog.Info("email sent", "type", "account_deleted", "to", email)
	}
	return err
}

// textToHTML converts plain text email to simple HTML format
// This improves deliverability, especially with Apple Mail servers
func textToHTML(text string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <pre style="white-space: pre-wrap; font-family: inherit;">%s</pre>
</body>
</html>`, text)
}
