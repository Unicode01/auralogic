package ticketbiz

import (
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
)

func ContentTooLong(max int) *bizerr.Error {
	return bizerr.Newf("ticket.contentTooLong", "Content cannot exceed %d characters", max).
		WithParams(map[string]interface{}{"max": max})
}

func StatusInvalid() *bizerr.Error {
	return bizerr.New("ticket.statusInvalid", "Invalid ticket status")
}

func PriorityInvalid() *bizerr.Error {
	return bizerr.New("ticket.priorityInvalid", "Invalid ticket priority")
}

func ClosedCannotSend() *bizerr.Error {
	return bizerr.New("ticket.closedCannotSend", "Ticket is closed, cannot send messages")
}

func ClosedCannotUpload() *bizerr.Error {
	return bizerr.New("ticket.closedCannotUpload", "Ticket is closed, cannot upload files")
}

func FileRequired() *bizerr.Error {
	return bizerr.New("ticket.fileRequired", "Please select a file")
}

func VoiceUploadDisabled() *bizerr.Error {
	return bizerr.New("ticket.voiceUploadDisabled", "Voice upload not allowed")
}

func AudioFormatInvalid() *bizerr.Error {
	return bizerr.New("ticket.audioFormatInvalid", "Invalid audio format")
}

func VoiceFileTooLarge(maxMB int64) *bizerr.Error {
	return bizerr.Newf("ticket.voiceFileTooLarge", "Voice file size cannot exceed %dMB", maxMB).
		WithParams(map[string]interface{}{"max": maxMB})
}

func ImageUploadDisabled() *bizerr.Error {
	return bizerr.New("ticket.imageUploadDisabled", "Image upload not allowed")
}

func ImageFileTooLarge(maxMB int64) *bizerr.Error {
	return bizerr.Newf("ticket.imageFileTooLarge", "Image size cannot exceed %dMB", maxMB).
		WithParams(map[string]interface{}{"max": maxMB})
}

func ImageFormatUnsupported() *bizerr.Error {
	return bizerr.New("ticket.imageFormatUnsupported", "Unsupported image format")
}

func ParseStatus(raw string) (models.TicketStatus, bool) {
	status := models.TicketStatus(strings.ToLower(strings.TrimSpace(raw)))
	switch status {
	case models.TicketStatusOpen,
		models.TicketStatusProcessing,
		models.TicketStatusResolved,
		models.TicketStatusClosed:
		return status, true
	default:
		return "", false
	}
}

func ParsePriority(raw string) (models.TicketPriority, bool) {
	priority := models.TicketPriority(strings.ToLower(strings.TrimSpace(raw)))
	switch priority {
	case models.TicketPriorityLow,
		models.TicketPriorityNormal,
		models.TicketPriorityHigh,
		models.TicketPriorityUrgent:
		return priority, true
	default:
		return "", false
	}
}

func BytesToMegabytes(size int64) int64 {
	if size <= 0 {
		return 0
	}
	const mb = 1024 * 1024
	return (size + mb - 1) / mb
}
