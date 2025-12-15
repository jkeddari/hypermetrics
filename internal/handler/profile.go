package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/jkeddari/hypermetrics/internal/ctxkeys"
	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/components/toast"
	"github.com/jkeddari/hypermetrics/internal/ui/layouts"
)

type ProfileHandler struct {
	profileService *service.ProfileService
}

func NewProfileHandler(profileService *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
	}
}

func (h *ProfileHandler) UpdateName(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	name := strings.TrimSpace(r.FormValue("name"))

	if name == "" {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Name is required",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	err := h.profileService.UpdateName(user.ID, name)
	if err != nil {
		slog.Error("failed to update name", "error", err, "user_id", user.ID)
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Failed to update name",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	profile, err := h.profileService.ByUserID(user.ID)
	if err != nil {
		slog.Error("failed to load profile", "error", err, "user_id", user.ID)
	}

	ui.RenderOOB(w, r, toast.Toast(toast.Props{
		Title:       "Success",
		Description: "Name updated successfully",
		Variant:     toast.VariantSuccess,
		Icon:        true,
		Dismissible: true,
	}), "beforeend:#toast-container")

	if profile != nil {
		ui.Render(w, r, layouts.AppSidebarDropdown(user, profile))
	}
}
