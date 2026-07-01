package service

import (
	"context"
	"strings"
	"time"

	"starcrystal/server/internal/config"
	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/store"
)

type InvitePendingContrib struct {
	PendingL1 float64 `json:"pendingDownlineL1Contrib"`
	PendingL2 float64 `json:"pendingDownlineL2Contrib"`
}

type InviteNotifyService struct {
	repo   store.PlayerRepository
	invite config.InviteConfig
	tz     string
	now    func() time.Time
}

func NewInviteNotifyService(repo store.PlayerRepository, invite config.InviteConfig, tz string) *InviteNotifyService {
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	return &InviteNotifyService{
		repo: repo, invite: invite, tz: tz,
		now: time.Now,
	}
}

func (s *InviteNotifyService) Enabled() bool {
	return s.invite.Enabled
}

func (s *InviteNotifyService) ComputePending(rec *store.AuthUserRecord) InvitePendingContrib {
	if rec == nil {
		return InvitePendingContrib{}
	}
	p1 := rec.CurDownlineL1Contrib - rec.NotifyWatermarkDownlineL1
	p2 := rec.CurDownlineL2Contrib - rec.NotifyWatermarkDownlineL2
	if p1 < 0 {
		p1 = 0
	}
	if p2 < 0 {
		p2 = 0
	}
	return InvitePendingContrib{PendingL1: p1, PendingL2: p2}
}

func inviteNotifyAutoAckEligible(now time.Time, autoAckDays int, since *time.Time, pending InvitePendingContrib) bool {
	if autoAckDays <= 0 {
		return false
	}
	if pending.PendingL1 <= 0 && pending.PendingL2 <= 0 {
		return false
	}
	if since == nil || since.IsZero() {
		return false
	}
	deadline := now.AddDate(0, 0, -autoAckDays)
	return !since.After(deadline)
}

func (s *InviteNotifyService) MaybeAutoAck(ctx context.Context, accountID string, rec *store.AuthUserRecord) {
	if s.repo == nil || rec == nil || s.invite.NotifyAutoAckDays <= 0 {
		return
	}
	pending := s.ComputePending(rec)
	if !inviteNotifyAutoAckEligible(s.now(), s.invite.NotifyAutoAckDays, rec.NotifyPendingSince, pending) {
		return
	}
	if err := s.repo.AckInviteNotifyWatermarks(ctx, accountID); err != nil {
		logger.Warn(logger.TopicAuth, "[invite-notify] auto-ack fail account=%s err=%v", accountID, err)
		return
	}
	logger.Info(logger.TopicAuth, "[invite-notify] auto-ack account=%s days=%d", accountID, s.invite.NotifyAutoAckDays)
}

func (s *InviteNotifyService) OnHeartbeat(ctx context.Context, accountID string) (InvitePendingContrib, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" || s.repo == nil {
		return InvitePendingContrib{}, nil
	}
	rec, err := s.repo.GetByAccountID(ctx, accountID)
	if err != nil {
		return InvitePendingContrib{}, err
	}
	pending := s.ComputePending(rec)
	if pending.PendingL1 > 0 || pending.PendingL2 > 0 {
		if rec.NotifyPendingSince == nil || rec.NotifyPendingSince.IsZero() {
			if err := s.repo.EnsureNotifyPendingSince(ctx, accountID); err != nil {
				return InvitePendingContrib{}, err
			}
			if rec, err = s.repo.GetByAccountID(ctx, accountID); err != nil {
				return InvitePendingContrib{}, err
			}
		}
	}
	s.MaybeAutoAck(ctx, accountID, rec)
	if rec, err = s.repo.GetByAccountID(ctx, accountID); err != nil {
		return InvitePendingContrib{}, err
	}
	return s.ComputePending(rec), nil
}

func (s *InviteNotifyService) Ack(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" || s.repo == nil {
		return nil
	}
	return s.repo.AckInviteNotifyWatermarks(ctx, accountID)
}
