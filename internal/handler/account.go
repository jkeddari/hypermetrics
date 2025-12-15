package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jkeddari/hypermetrics/internal/ctxkeys"
	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/components/toast"
	"github.com/jkeddari/hypermetrics/internal/ui/layouts"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
	"github.com/jkeddari/hypermetrics/internal/validation"
)

type AccountHandler struct {
	authService *service.AuthService
	userService *service.UserService
	fileService *service.FileService
}

func NewAccountHandler(authService *service.AuthService, userService *service.UserService, fileService *service.FileService) *AccountHandler {
	return &AccountHandler{
		authService: authService,
		userService: userService,
		fileService: fileService,
	}
}

func (h *AccountHandler) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	email := r.FormValue("email")
	email = strings.TrimSpace(strings.ToLower(email))

	err := validation.ValidateEmail(email)
	if err != nil {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Valid email is required",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	if email == user.Email {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Info",
			Description: "Email address is already set to this value",
			Variant:     toast.VariantInfo,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	err = h.authService.RequestEmailChange(user.ID, email)
	if err != nil {
		slog.Warn("email change request failed", "error", err, "user_id", user.ID, "new_email", email)

		errMsg := "Failed to change email"
		if errors.Is(err, service.ErrEmailAlreadyExists) {
			errMsg = "Email already in use"
		} else if errors.Is(err, service.ErrInvalidEmail) {
			errMsg = "Invalid email address"
		}

		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: errMsg,
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	slog.Info("email change requested", "user_id", user.ID, "old_email", user.Email, "new_email", email)
	ui.RenderOOB(w, r, toast.Toast(toast.Props{
		Title:       "Verification Required",
		Description: "Please check your new email address to verify the change",
		Variant:     toast.VariantInfo,
		Icon:        true,
		Dismissible: true,
	}), "beforeend:#toast-container")
}

func (h *AccountHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if currentPassword == "" || newPassword == "" || confirmPassword == "" {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "All password fields are required",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	if newPassword != confirmPassword {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "New passwords do not match",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	err := validation.ValidatePassword(newPassword)
	if err != nil {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: err.Error(),
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	err = h.userService.UpdatePassword(user.ID, currentPassword, newPassword)
	if err != nil {
		slog.Warn("password update failed", "error", err, "user_id", user.ID)

		errMsg := "Failed to update password"
		if errors.Is(err, service.ErrInvalidCurrentPassword) {
			errMsg = "Current password is incorrect"
		}

		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: errMsg,
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	slog.Info("password updated", "user_id", user.ID)
	ui.RenderOOB(w, r, toast.Toast(toast.Props{
		Title:       "Success",
		Description: "Password updated successfully",
		Variant:     toast.VariantSuccess,
		Icon:        true,
		Dismissible: true,
	}), "beforeend:#toast-container")
	ui.RenderFragment(w, r, pages.SettingsPasswordSection(), "settings-password-form")
}

func (h *AccountHandler) SetPassword(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if newPassword == "" || confirmPassword == "" {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "All fields are required",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	if newPassword != confirmPassword {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Passwords do not match",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	err := validation.ValidatePassword(newPassword)
	if err != nil {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: err.Error(),
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	err = h.authService.SetPassword(user.ID, newPassword)
	if err != nil {
		slog.Warn("set password failed", "error", err, "user_id", user.ID)

		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Failed to Set Password",
			Description: err.Error(),
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
			Duration:    8000, // 8 seconds for errors
		}), "beforeend:#toast-container")
		return
	}

	// Reload user from DB to get updated password_hash
	updatedUser, err := h.userService.ByID(user.ID)
	if err != nil {
		slog.Error("failed to reload user after password set", "error", err, "user_id", user.ID)
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Password set but failed to refresh. Please reload the page.",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	// Update context with fresh user
	ctx := ctxkeys.WithUser(r.Context(), updatedUser)

	slog.Info("password set", "user_id", user.ID)
	ui.RenderOOB(w, r, toast.Toast(toast.Props{
		Title:       "Success",
		Description: "Password set successfully. You can now sign in with your password.",
		Variant:     toast.VariantSuccess,
		Icon:        true,
		Dismissible: true,
	}), "beforeend:#toast-container")
	ui.RenderFragment(w, r.WithContext(ctx), pages.SettingsPasswordSection(), "settings-password-form")
}

func (h *AccountHandler) RemovePassword(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	err := h.authService.RemovePassword(user.ID)
	if err != nil {
		slog.Warn("remove password failed", "error", err, "user_id", user.ID)

		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Failed to Remove Password",
			Description: err.Error(),
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
			Duration:    8000, // 8 seconds for errors
		}), "beforeend:#toast-container")
		return
	}

	// Reload user from DB to get updated password_hash (now NULL)
	updatedUser, err := h.userService.ByID(user.ID)
	if err != nil {
		slog.Error("failed to reload user after password removal", "error", err, "user_id", user.ID)
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Password removed but failed to refresh. Please reload the page.",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	// Update context with fresh user
	ctx := ctxkeys.WithUser(r.Context(), updatedUser)

	slog.Info("password removed", "user_id", user.ID)

	ui.RenderOOB(w, r, toast.Toast(toast.Props{
		Title:       "Success",
		Description: "Password removed. You can now only sign in with magic links.",
		Variant:     toast.VariantSuccess,
		Icon:        true,
		Dismissible: true,
	}), "beforeend:#toast-container")
	ui.RenderFragment(w, r.WithContext(ctx), pages.SettingsPasswordSection(), "settings-password-form")
}

func (h *AccountHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	confirmation := strings.TrimSpace(r.FormValue("delete_confirmation"))

	if confirmation != "DELETE" {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Please type DELETE to confirm account deletion",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	err := h.userService.DeleteAccount(user.ID)
	if err != nil {
		// Handle active subscription error with specific message
		if errors.Is(err, service.ErrActiveSubscription) {
			slog.Warn("account deletion failed: active subscription", "error", err, "user_id", user.ID)
			ui.RenderOOB(w, r, toast.Toast(toast.Props{
				Title:       "Active Subscription",
				Description: "Please cancel your subscription before deleting your account. Visit billing to manage your subscription.",
				Variant:     toast.VariantError,
				Icon:        true,
				Dismissible: true,
			}), "beforeend:#toast-container")
			return
		}

		// Unexpected error - log as error for Sentry
		slog.Error("account deletion failed", "error", err, "user_id", user.ID)
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Failed to delete account. Please try again.",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	slog.Info("account deleted", "user_id", user.ID, "email", user.Email)
	h.authService.ClearJWTCookie(w)

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (h *AccountHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())
	profile := ctxkeys.Profile(r.Context())

	// Parse multipart form (10MB max)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Failed to parse form",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "No file uploaded",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}
	defer func() {
		closeErr := file.Close()
		if closeErr != nil {
			slog.Error("failed to close file", "error", closeErr)
		}
	}()

	err = validation.ValidateFile(header, validation.ImageConstraints)
	if err != nil {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: err.Error(),
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	// Explicitly delete old avatar before uploading new one
	err = h.fileService.DeleteUserAvatar(user.ID)
	if err != nil {
		slog.Warn("failed to delete old avatar", "error", err, "user_id", user.ID)
		// Continue anyway - we'll upload the new one
	}

	_, err = h.fileService.Upload(user.ID, "user", user.ID, "avatar", file, header, true) // Avatars are public
	if err != nil {
		slog.Error("failed to upload avatar", "error", err, "user_id", user.ID)
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Failed to upload avatar",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	// Reload user to get updated avatar URL
	updatedUser, err := h.userService.ByID(user.ID)
	if err != nil {
		slog.Error("failed to reload user", "error", err, "user_id", user.ID)
	}

	ui.RenderOOB(w, r, toast.Toast(toast.Props{
		Title:       "Success",
		Description: "Avatar uploaded successfully",
		Variant:     toast.VariantSuccess,
		Icon:        true,
		Dismissible: true,
	}), "beforeend:#toast-container")

	if updatedUser != nil {
		ui.RenderFragment(w, r, pages.SettingsAvatarSection(updatedUser), "settings-avatar-display")
		ui.RenderFragment(w, r, pages.SettingsAvatarSection(updatedUser), "settings-avatar-form")
		ui.Render(w, r, layouts.AppSidebarDropdown(updatedUser, profile))
	}
}

func (h *AccountHandler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())
	profile := ctxkeys.Profile(r.Context())

	err := h.fileService.DeleteUserAvatar(user.ID)
	if err != nil {
		slog.Error("failed to delete avatar", "error", err, "user_id", user.ID)
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Failed to delete avatar",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	// Reload user to clear avatar URL
	updatedUser, err := h.userService.ByID(user.ID)
	if err != nil {
		slog.Error("failed to reload user", "error", err, "user_id", user.ID)
	}

	ui.RenderOOB(w, r, toast.Toast(toast.Props{
		Title:       "Success",
		Description: "Avatar removed successfully",
		Variant:     toast.VariantSuccess,
		Icon:        true,
		Dismissible: true,
	}), "beforeend:#toast-container")

	if updatedUser != nil {
		ui.RenderFragment(w, r, pages.SettingsAvatarSection(updatedUser), "settings-avatar-display")
		ui.RenderFragment(w, r, pages.SettingsAvatarSection(updatedUser), "settings-avatar-form")
		ui.Render(w, r, layouts.AppSidebarDropdown(updatedUser, profile))
	}
}
