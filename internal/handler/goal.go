package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jkeddari/hypermetrics/internal/ctxkeys"
	"github.com/jkeddari/hypermetrics/internal/model"
	"github.com/jkeddari/hypermetrics/internal/service"
	"github.com/jkeddari/hypermetrics/internal/ui"
	"github.com/jkeddari/hypermetrics/internal/ui/components/toast"
	"github.com/jkeddari/hypermetrics/internal/ui/pages"
)

type GoalHandler struct {
	goalService *service.GoalService
}

func NewGoalHandler(goalService *service.GoalService) *GoalHandler {
	return &GoalHandler{
		goalService: goalService,
	}
}

func (h *GoalHandler) GoalsPage(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "recent"
	}

	goals, err := h.goalService.Goals(user.ID, sortBy)
	if err != nil {
		slog.Error("failed to get goals", "error", err, "user_id", user.ID)
		http.Error(w, "Failed to load goals", http.StatusInternalServerError)
		return
	}

	// If HTMX request, only render the content portion
	if r.Header.Get("HX-Request") == "true" {
		ui.Render(w, r, pages.GoalsContent(goals, sortBy))
		return
	}

	ui.Render(w, r, pages.Goals(goals, sortBy))
}

func (h *GoalHandler) GoalDetailPage(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	goalID := r.PathValue("id")

	goal, entries, err := h.goalService.GoalWithEntries(user.ID, goalID)
	if err != nil {
		slog.Error("failed to get goal", "error", err, "user_id", user.ID, "goal_id", goalID)
		http.Error(w, "Goal not found", http.StatusNotFound)
		return
	}

	ui.Render(w, r, pages.GoalDetail(goal, entries))
}

func (h *GoalHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	title := r.FormValue("title")
	description := r.FormValue("description")

	if title == "" {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Title is required",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	_, err := h.goalService.Create(user.ID, title, description)
	if err == service.ErrGoalLimitReached {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Upgrade Required",
			Description: "Upgrade to Pro for unlimited goals",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	if err != nil {
		slog.Error("failed to create goal", "error", err, "user_id", user.ID)
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Failed to create goal",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "recent"
	}

	goals, err := h.goalService.Goals(user.ID, sortBy)
	if err != nil {
		slog.Error("failed to reload goals", "error", err, "user_id", user.ID)
		goals = []*model.Goal{} // Fallback to empty list
	}

	ui.RenderOOB(w, r, toast.Toast(toast.Props{
		Title:       "Success",
		Description: "Goal created successfully",
		Variant:     toast.VariantSuccess,
		Icon:        true,
		Dismissible: true,
	}), "beforeend:#toast-container")

	ui.Render(w, r, pages.GoalsContent(goals, sortBy))
}

func (h *GoalHandler) CompleteEntry(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	goalID := r.PathValue("id")
	stepStr := r.PathValue("step")

	step, err := strconv.Atoi(stepStr)
	if err != nil || step < 1 || step > 100 {
		http.Error(w, "Invalid step number", http.StatusBadRequest)
		return
	}

	err = h.goalService.CompleteEntry(user.ID, goalID, step)
	if err == service.ErrInvalidStep {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "You must complete the previous steps first",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	if err == service.ErrGoalAlreadyCompleted {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Info",
			Description: "This goal is already completed",
			Variant:     toast.VariantInfo,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	if err != nil {
		slog.Error("failed to complete entry", "error", err, "user_id", user.ID, "goal_id", goalID, "step", step)
		http.Error(w, "Failed to complete entry", http.StatusInternalServerError)
		return
	}

	goal, entries, err := h.goalService.GoalWithEntries(user.ID, goalID)
	if err != nil {
		slog.Error("failed to reload goal", "error", err, "user_id", user.ID, "goal_id", goalID)
		http.Error(w, "Failed to reload goal", http.StatusInternalServerError)
		return
	}

	ui.Render(w, r, pages.GoalDetailContent(goal, entries))
}

func (h *GoalHandler) EntryDialog(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	goalID := r.PathValue("id")
	stepStr := r.PathValue("step")

	step, err := strconv.Atoi(stepStr)
	if err != nil || step < 1 || step > 100 {
		http.Error(w, "Invalid step number", http.StatusBadRequest)
		return
	}

	// Get goal for ownership check
	goal, err := h.goalService.ByID(user.ID, goalID)
	if err != nil {
		http.Error(w, "Goal not found", http.StatusNotFound)
		return
	}

	// Get specific entry
	entry, err := h.goalService.EntryByGoalAndStep(goalID, step)
	if err != nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	if !entry.Completed {
		http.Error(w, "Entry not completed", http.StatusNotFound)
		return
	}

	ui.Render(w, r, pages.GoalEntryDialog(goal, entry))
}

func (h *GoalHandler) UpdateEntry(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	goalID := r.PathValue("id")
	stepStr := r.PathValue("step")

	step, err := strconv.Atoi(stepStr)
	if err != nil || step < 1 || step > 100 {
		http.Error(w, "Invalid step number", http.StatusBadRequest)
		return
	}

	note := r.FormValue("note")
	completedAtStr := r.FormValue("completed_at")

	// Parse completed_at if provided (datepicker returns YYYY-MM-DD format)
	var completedAt *time.Time
	if completedAtStr != "" {
		parsed, err := time.Parse("2006-01-02", completedAtStr)
		if err != nil {
			slog.Error("failed to parse completed_at", "error", err, "value", completedAtStr)
			http.Error(w, "Invalid date format", http.StatusBadRequest)
			return
		}
		completedAt = &parsed
	}

	err = h.goalService.UpdateEntry(user.ID, goalID, step, note, completedAt)
	if err != nil {
		slog.Error("failed to update entry", "error", err, "user_id", user.ID, "goal_id", goalID, "step", step)
		http.Error(w, "Failed to update entry", http.StatusInternalServerError)
		return
	}

	goal, entries, err := h.goalService.GoalWithEntries(user.ID, goalID)
	if err != nil {
		slog.Error("failed to reload goal", "error", err, "user_id", user.ID, "goal_id", goalID)
		http.Error(w, "Failed to reload goal", http.StatusInternalServerError)
		return
	}

	ui.Render(w, r, pages.GoalDetailContent(goal, entries))
	ui.RenderOOB(w, r, toast.Toast(toast.Props{
		Title:       "Success",
		Description: "Entry updated successfully",
		Variant:     toast.VariantSuccess,
		Icon:        true,
		Dismissible: true,
	}), "beforeend:#toast-container")
}

func (h *GoalHandler) UncompleteEntry(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	goalID := r.PathValue("id")
	stepStr := r.PathValue("step")

	step, err := strconv.Atoi(stepStr)
	if err != nil || step < 1 || step > 100 {
		http.Error(w, "Invalid step number", http.StatusBadRequest)
		return
	}

	err = h.goalService.UncompleteEntry(user.ID, goalID, step)
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

	goal, entries, err := h.goalService.GoalWithEntries(user.ID, goalID)
	if err != nil {
		slog.Error("failed to reload goal", "error", err, "user_id", user.ID, "goal_id", goalID)
		http.Error(w, "Failed to reload goal", http.StatusInternalServerError)
		return
	}

	ui.Render(w, r, pages.GoalDetailContent(goal, entries))
	ui.RenderOOB(w, r, toast.Toast(toast.Props{
		Title:       "Success",
		Description: "Entry uncompleted successfully",
		Variant:     toast.VariantSuccess,
		Icon:        true,
		Dismissible: true,
	}), "beforeend:#toast-container")
}

func (h *GoalHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	goalID := r.PathValue("id")
	title := r.FormValue("title")
	description := r.FormValue("description")

	if title == "" {
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Title is required",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	goal, err := h.goalService.ByID(user.ID, goalID)
	if err != nil {
		slog.Error("failed to get goal", "error", err, "user_id", user.ID, "goal_id", goalID)
		http.Error(w, "Goal not found", http.StatusNotFound)
		return
	}

	err = h.goalService.Update(user.ID, goalID, title, description, goal.Status)
	if err != nil {
		slog.Error("failed to update goal", "error", err, "user_id", user.ID, "goal_id", goalID)
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Failed to update goal",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	goal, entries, err := h.goalService.GoalWithEntries(user.ID, goalID)
	if err != nil {
		slog.Error("failed to reload goal", "error", err, "user_id", user.ID, "goal_id", goalID)
		http.Error(w, "Failed to reload goal", http.StatusInternalServerError)
		return
	}

	ui.Render(w, r, pages.GoalDetailContent(goal, entries))
	ui.RenderOOB(w, r, toast.Toast(toast.Props{
		Title:       "Success",
		Description: "Goal updated successfully",
		Variant:     toast.VariantSuccess,
		Icon:        true,
		Dismissible: true,
	}), "beforeend:#toast-container")
}

func (h *GoalHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	goalID := r.PathValue("id")

	err := h.goalService.Delete(user.ID, goalID)
	if err != nil {
		slog.Error("failed to delete goal", "error", err, "user_id", user.ID, "goal_id", goalID)
		ui.RenderOOB(w, r, toast.Toast(toast.Props{
			Title:       "Error",
			Description: "Failed to delete goal",
			Variant:     toast.VariantError,
			Icon:        true,
			Dismissible: true,
		}), "beforeend:#toast-container")
		return
	}

	w.Header().Set("HX-Redirect", "/app/goals")
	w.WriteHeader(http.StatusOK)
}

func (h *GoalHandler) EditDialog(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	goalID := r.PathValue("id")

	goal, err := h.goalService.ByID(user.ID, goalID)
	if err != nil {
		slog.Error("failed to get goal", "error", err, "user_id", user.ID, "goal_id", goalID)
		http.Error(w, "Goal not found", http.StatusNotFound)
		return
	}

	ui.Render(w, r, pages.GoalEditDialog(goal))
}

func (h *GoalHandler) DeleteDialog(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())

	goalID := r.PathValue("id")

	goal, err := h.goalService.ByID(user.ID, goalID)
	if err != nil {
		slog.Error("failed to get goal", "error", err, "user_id", user.ID, "goal_id", goalID)
		http.Error(w, "Goal not found", http.StatusNotFound)
		return
	}

	ui.Render(w, r, pages.GoalDeleteDialog(goal))
}

func (h *GoalHandler) Export(w http.ResponseWriter, r *http.Request) {
	user := ctxkeys.User(r.Context())
	subscription := ctxkeys.Subscription(r.Context())

	if !subscription.HasFeature(model.FeatureExport) {
		http.Error(w, "Upgrade to Pro to export your goals", http.StatusForbidden)
		return
	}

	goals, err := h.goalService.Goals(user.ID, "")
	if err != nil {
		slog.Error("failed to list goals for export", "error", err, "user_id", user.ID)
		http.Error(w, "Failed to export goals", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=goals-export.json")

	err = json.NewEncoder(w).Encode(goals)
	if err != nil {
		slog.Error("failed to encode goals", "error", err, "user_id", user.ID)
		http.Error(w, "Failed to export goals", http.StatusInternalServerError)
		return
	}
}
