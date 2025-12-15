package handler

import (
	"net/http"

	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/components/toast"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type SettingsHandler struct{}

func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{}
}

func (h *SettingsHandler) SettingsPage(w http.ResponseWriter, r *http.Request) {
	// Check if coming from forgot password flow
	if r.URL.Query().Get("password_removed") == "1" {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Password Removed",
			Description: "For security, your password was removed. Set a new one below if you'd like.",
			Variant:     toast.VariantInfo,
			Icon:        true,
			Dismissible: true,
			Duration:    8000,
		}), "beforeend:#toast-container")
	}

	ui.Render(w, r, pages.Settings())
}
