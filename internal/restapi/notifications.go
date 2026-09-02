package restapi

import (
	"context"
	"io"
	"log"
	"net/http"
)

type NotificationService interface {
	ListAndUploadNotifications(context.Context) error
}

type notifications struct {
	service NotificationService
}

func (h *notifications) handleRoutes(handle func(string, func(http.ResponseWriter, *http.Request))) {
	handle("POST /github/notifications", withOverriddenCancel(h.postNotificationsUpload))
}

func (h *notifications) postNotificationsUpload(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ListAndUploadNotifications(r.Context()); err != nil {
		log.Printf("Failed to dump notifications: %v", err)
		http.Error(w, "failed to list and upload notifications", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}
