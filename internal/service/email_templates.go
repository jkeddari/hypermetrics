package service

import "fmt"

func forgotPasswordEmailTemplate(signInURL, appName string) (string, string) {
	subject := fmt.Sprintf("Reset your password for %s", appName)
	body := fmt.Sprintf(`You requested to reset your password. For security, we'll remove your password and sign you in with this link:
%s

After signing in, you can set a new password in Settings.

This link expires in 10 minutes and can only be used once.

If you didn't request this, you can safely ignore this email. Your password won't be changed.

Best,
The %s Team`, signInURL, appName)

	return subject, body
}

func magicLinkEmailTemplate(magicURL, appName string) (string, string) {
	subject := fmt.Sprintf("Sign in to %s", appName)
	body := fmt.Sprintf(`Click this link to sign in to your account:
%s

This link expires in 10 minutes and can only be used once.

If you didn't request this, ignore this email.

Best,
The %s Team`, magicURL, appName)

	return subject, body
}

func welcomeEmailTemplate(name, dashboardURL, appName string) (string, string) {
	subject := fmt.Sprintf("Welcome to %s!", appName)
	body := fmt.Sprintf(`Hi %s,

Your email is verified and your account is active!

Get started: %s

If you have questions, reach out to our support team.

Best,
The %s Team`, name, dashboardURL, appName)

	return subject, body
}

func emailChangeVerificationTemplate(name, verifyURL, appName string) (string, string) {
	subject := fmt.Sprintf("Verify your new email for %s", appName)
	body := fmt.Sprintf(`Hi %s,

You requested to change your email address. Please verify your new email by clicking this link:
%s

This link expires in 24 hours.

If you didn't request this change, you can safely ignore this email.

Best,
The %s Team`, name, verifyURL, appName)

	return subject, body
}

func emailChangeNotificationTemplate(name, newEmail, appName string) (string, string) {
	subject := fmt.Sprintf("Email change requested for %s", appName)
	body := fmt.Sprintf(`Hi %s,

A request was made to change your email address to: %s

If this was you, please verify the new email address by clicking the link we sent to it.

If you didn't request this change, your account may be compromised. Please secure your account immediately by changing your password.

Best,
The %s Team`, name, newEmail, appName)

	return subject, body
}

func accountDeletedEmailTemplate(name, appName string) (string, string) {
	subject := fmt.Sprintf("Your %s account has been deleted", appName)
	body := fmt.Sprintf(`Hi %s,

Your account has been permanently deleted from %s.

All your data, including your profile, files, and settings, has been removed from our systems.

If you didn't request this deletion, please contact our support team immediately, though we won't be able to recover your account.

We're sorry to see you go. If you change your mind, you're welcome to create a new account anytime.

Best,
The %s Team`, name, appName, appName)

	return subject, body
}
