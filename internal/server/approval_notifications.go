package server

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"ai-shortlink/internal/model"
	"ai-shortlink/internal/store"
)

type approvalNotification struct {
	ResourceType   string
	ResourceID     int64
	OwnerAccountID int64
	Title          string
	Code           string
	ParentTitle    string
	PublicURL      string
	Decision       string
	ReviewNote     string
	ReviewedAt     time.Time
	RecipientName  string
}

func reviewDecisionChanged(before, after string) bool {
	before = strings.TrimSpace(before)
	after = strings.TrimSpace(after)
	return before != after && (after == "approved" || after == "rejected")
}

func reviewTime(approvedAt, reviewedAt *time.Time) time.Time {
	if reviewedAt != nil {
		return *reviewedAt
	}
	if approvedAt != nil {
		return *approvedAt
	}
	return time.Now()
}

func (s *Server) notifyShortLinkReviewed(before, after *model.ShortLink, publicURL string) {
	if before == nil || after == nil || !reviewDecisionChanged(before.ApprovalStatus, after.ApprovalStatus) {
		return
	}
	s.sendApprovalNotificationAsync(approvalNotification{
		ResourceType:   "short_link",
		ResourceID:     after.ID,
		OwnerAccountID: after.OwnerAccountID,
		Title:          firstNonEmpty(after.Title, after.Code),
		Code:           after.Code,
		PublicURL:      publicURL,
		Decision:       after.ApprovalStatus,
		ReviewNote:     after.ReviewNote,
		ReviewedAt:     reviewTime(after.ApprovedAt, after.ReviewedAt),
	})
}

func (s *Server) notifyLiveQRReviewed(before, after *model.LiveQR, publicURL string) {
	if before == nil || after == nil || !reviewDecisionChanged(before.ApprovalStatus, after.ApprovalStatus) {
		return
	}
	s.sendApprovalNotificationAsync(approvalNotification{
		ResourceType:   "live_qr",
		ResourceID:     after.ID,
		OwnerAccountID: after.OwnerAccountID,
		Title:          firstNonEmpty(after.Title, after.Code),
		Code:           after.Code,
		PublicURL:      publicURL,
		Decision:       after.ApprovalStatus,
		ReviewNote:     after.ReviewNote,
		ReviewedAt:     reviewTime(after.ApprovedAt, after.ReviewedAt),
	})
}

func (s *Server) notifyLiveQRItemReviewed(before, after *model.LiveQRItem, live *model.LiveQR, publicURL string) {
	if before == nil || after == nil || live == nil || !reviewDecisionChanged(before.ApprovalStatus, after.ApprovalStatus) {
		return
	}
	s.sendApprovalNotificationAsync(approvalNotification{
		ResourceType:   "live_qr_item",
		ResourceID:     after.ID,
		OwnerAccountID: live.OwnerAccountID,
		Title:          firstNonEmpty(after.Title, after.QRImageURL),
		Code:           live.Code,
		ParentTitle:    firstNonEmpty(live.Title, live.Code),
		PublicURL:      publicURL,
		Decision:       after.ApprovalStatus,
		ReviewNote:     after.ReviewNote,
		ReviewedAt:     reviewTime(after.ApprovedAt, after.ReviewedAt),
	})
}

func (s *Server) sendApprovalNotificationAsync(n approvalNotification) {
	if n.OwnerAccountID <= 0 || strings.TrimSpace(n.PublicURL) == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := s.sendApprovalNotification(ctx, n); err != nil {
			log.Printf("review notification %s:%d (%s): %v", n.ResourceType, n.ResourceID, n.Decision, err)
		}
	}()
}

func (s *Server) sendApprovalNotification(ctx context.Context, n approvalNotification) error {
	st := s.settings(ctx)
	if !approvalSMTPReady(st) {
		return nil
	}
	acct, err := s.store().GetAdminAccount(ctx, n.OwnerAccountID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if acct.Status != "active" || !validEmail(acct.Email) {
		return nil
	}
	n.RecipientName = firstNonEmpty(acct.Name, emailName(acct.Email))
	return s.sendApprovalNotificationMail(ctx, acct.Email, n)
}

func approvalSMTPReady(st model.SystemSettings) bool {
	return st.SMTPEnabled &&
		strings.TrimSpace(st.SMTPHost) != "" &&
		strings.TrimSpace(st.SMTPFrom) != "" &&
		st.SMTPPort > 0 &&
		st.SMTPPasswordSet
}
