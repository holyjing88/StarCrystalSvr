package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLPlayerRepository struct {
	mu  sync.Mutex
	dsn string
	db  *sql.DB
}

func NewMySQLPlayerRepository(mysqlDSN string) (*MySQLPlayerRepository, error) {
	if mysqlDSN == "" {
		return nil, errors.New("mysql dsn is empty")
	}
	s := &MySQLPlayerRepository{dsn: mysqlDSN}
	if err := s.reconnect(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *MySQLPlayerRepository) GetByAccountID(ctx context.Context, accountID string) (*AuthUserRecord, error) {
	const q = `
SELECT 
  account_id,
  account_type,
  account_value,
  COALESCE(email,''),
  COALESCE(phone,''),
  COALESCE(password_hash,''),
  COALESCE(display_name,''),
  COALESCE(nickname,''),
  COALESCE(provider,''),
  COALESCE(invited_user_id,''),
  COALESCE(device_id,''),
  COALESCE(registration_ip,''),
  COALESCE(ad_rewards_disabled, 0),
  COALESCE(status, 1),
  COALESCE(ban_reason, ''),
  device_silent_until,
  COALESCE(curgold, 0),
  COALESCE(totalgold, 0),
  COALESCE(curtoken, 0),
  COALESCE(totaltoken, 0),
  COALESCE(cur_direct_inviter_share, 0),
  COALESCE(total_direct_inviter_share, 0),
  COALESCE(cur_second_inviter_share, 0),
  COALESCE(total_second_inviter_share, 0),
  COALESCE(cur_downline_l1_contrib, 0),
  COALESCE(total_downline_l1_contrib, 0),
  COALESCE(cur_downline_l2_contrib, 0),
  COALESCE(total_downline_l2_contrib, 0),
  COALESCE(notify_watermark_downline_l1, 0),
  COALESCE(notify_watermark_downline_l2, 0),
  notify_pending_since
FROM auth_accounts
WHERE account_id = ? AND deleted_at IS NULL
LIMIT 1`
	var out *AuthUserRecord
	err := s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, q, accountID)
		rec := &AuthUserRecord{}
		var accountIDValue string
		var deviceSilentUntil sql.NullTime
		var notifyPendingSince sql.NullTime
		if err := row.Scan(
			&accountIDValue,
			&rec.AccountType,
			&rec.AccountValue,
			&rec.Email,
			&rec.Phone,
			&rec.PasswordHash,
			&rec.DisplayName,
			&rec.Nickname,
			&rec.Provider,
			&rec.InvitedUserID,
			&rec.DeviceID,
			&rec.RegistrationIP,
			&rec.AdRewardsDisabled,
			&rec.Status,
			&rec.BanReason,
			&deviceSilentUntil,
			&rec.CurGold,
			&rec.TotalGold,
			&rec.CurToken,
			&rec.TotalToken,
			&rec.CurDirectInviterShare,
			&rec.TotalDirectInviterShare,
			&rec.CurSecondInviterShare,
			&rec.TotalSecondInviterShare,
			&rec.CurDownlineL1Contrib,
			&rec.TotalDownlineL1Contrib,
			&rec.CurDownlineL2Contrib,
			&rec.TotalDownlineL2Contrib,
			&rec.NotifyWatermarkDownlineL1,
			&rec.NotifyWatermarkDownlineL2,
			&notifyPendingSince,
		); err != nil {
			return err
		}
		rec.UserID = strings.TrimSpace(accountIDValue)
		rec.AccountID = rec.UserID
		if deviceSilentUntil.Valid {
			t := deviceSilentUntil.Time
			rec.DeviceSilentUntil = &t
		}
		if notifyPendingSince.Valid {
			t := notifyPendingSince.Time
			rec.NotifyPendingSince = &t
		}
		if notifyPendingSince.Valid {
			t := notifyPendingSince.Time
			rec.NotifyPendingSince = &t
		}
		out = rec
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return out, nil
}

const inviteCodeInsertMaxRetries = 100

func (s *MySQLPlayerRepository) DeleteAuthAccountCascade(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil
	}
	return s.withRetry(ctx, func(db *sql.DB) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		// accountid 无外键：用户作为邀请人时的关系行需手动删；作为被邀请人时可在删除 auth_accounts 时由 FK CASCADE 清理。
		if _, err = tx.ExecContext(ctx, `DELETE FROM auth_invite_members WHERE accountid = ?`, accountID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM auth_accounts WHERE account_id = ? LIMIT 1`, accountID)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return tx.Commit()
	})
}

func (s *MySQLPlayerRepository) FindLatestAccountByDeviceID(ctx context.Context, deviceID string) (*AuthUserRecord, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, nil
	}
	const q = `
SELECT
  account_id,
  account_id,
  account_type,
  account_value,
  COALESCE(email,''),
  COALESCE(phone,''),
  COALESCE(password_hash,''),
  COALESCE(display_name,''),
  COALESCE(provider,''),
  COALESCE(invited_user_id,''),
  COALESCE(device_id,''),
  COALESCE(registration_ip,''),
  COALESCE(ad_rewards_disabled, 0),
  COALESCE(status, 1),
  COALESCE(ban_reason, ''),
  device_silent_until,
  COALESCE(curgold, 0),
  COALESCE(totalgold, 0),
  COALESCE(curtoken, 0),
  COALESCE(totaltoken, 0),
  COALESCE(cur_direct_inviter_share, 0),
  COALESCE(total_direct_inviter_share, 0),
  COALESCE(cur_second_inviter_share, 0),
  COALESCE(total_second_inviter_share, 0)
FROM auth_accounts
WHERE deleted_at IS NULL AND device_id = ?
ORDER BY updated_at DESC
LIMIT 1`
	var out *AuthUserRecord
	err := s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, q, deviceID)
		rec := &AuthUserRecord{}
		var deviceSilentUntil sql.NullTime
		var notifyPendingSince sql.NullTime
		if err := row.Scan(
			&rec.UserID,
			&rec.AccountID,
			&rec.AccountType,
			&rec.AccountValue,
			&rec.Email,
			&rec.Phone,
			&rec.PasswordHash,
			&rec.DisplayName,
			&rec.Provider,
			&rec.InvitedUserID,
			&rec.DeviceID,
			&rec.RegistrationIP,
			&rec.AdRewardsDisabled,
			&rec.Status,
			&rec.BanReason,
			&deviceSilentUntil,
			&rec.CurGold,
			&rec.TotalGold,
			&rec.CurToken,
			&rec.TotalToken,
			&rec.CurDirectInviterShare,
			&rec.TotalDirectInviterShare,
			&rec.CurSecondInviterShare,
			&rec.TotalSecondInviterShare,
		); err != nil {
			return err
		}
		if deviceSilentUntil.Valid {
			t := deviceSilentUntil.Time
			rec.DeviceSilentUntil = &t
		}
		if notifyPendingSince.Valid {
			t := notifyPendingSince.Time
			rec.NotifyPendingSince = &t
		}
		if notifyPendingSince.Valid {
			t := notifyPendingSince.Time
			rec.NotifyPendingSince = &t
		}
		out = rec
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (s *MySQLPlayerRepository) FindLatestActiveAccountByDeviceID(ctx context.Context, deviceID string) (*AuthUserRecord, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, nil
	}
	const q = `
SELECT
  a.account_id,
  a.account_id,
  a.account_type,
  a.account_value,
  COALESCE(a.email,''),
  COALESCE(a.phone,''),
  COALESCE(a.password_hash,''),
  COALESCE(a.display_name,''),
  COALESCE(a.nickname,''),
  COALESCE(a.provider,''),
  COALESCE(a.invited_user_id,''),
  COALESCE(a.device_id,''),
  COALESCE(a.registration_ip,''),
  COALESCE(a.ad_rewards_disabled, 0),
  COALESCE(a.status, 1),
  COALESCE(a.ban_reason, ''),
  a.device_silent_until,
  COALESCE(a.curgold, 0),
  COALESCE(a.totalgold, 0),
  COALESCE(a.curtoken, 0),
  COALESCE(a.totaltoken, 0),
  COALESCE(a.cur_direct_inviter_share, 0),
  COALESCE(a.total_direct_inviter_share, 0),
  COALESCE(a.cur_second_inviter_share, 0),
  COALESCE(a.total_second_inviter_share, 0),
  COALESCE(a.cur_downline_l1_contrib, 0),
  COALESCE(a.total_downline_l1_contrib, 0),
  COALESCE(a.cur_downline_l2_contrib, 0),
  COALESCE(a.total_downline_l2_contrib, 0),
  COALESCE(a.notify_watermark_downline_l1, 0),
  COALESCE(a.notify_watermark_downline_l2, 0),
  a.notify_pending_since
FROM auth_device_account_map dm
JOIN auth_accounts a
  ON a.account_id = dm.account_id
WHERE a.deleted_at IS NULL AND dm.device_id = ? AND COALESCE(a.status,1) <> 2
ORDER BY a.updated_at DESC
LIMIT 1`
	var out *AuthUserRecord
	err := s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, q, deviceID)
		rec := &AuthUserRecord{}
		var deviceSilentUntil sql.NullTime
		var notifyPendingSince sql.NullTime
		if err := row.Scan(
			&rec.UserID,
			&rec.AccountID,
			&rec.AccountType,
			&rec.AccountValue,
			&rec.Email,
			&rec.Phone,
			&rec.PasswordHash,
			&rec.DisplayName,
			&rec.Nickname,
			&rec.Provider,
			&rec.InvitedUserID,
			&rec.DeviceID,
			&rec.RegistrationIP,
			&rec.AdRewardsDisabled,
			&rec.Status,
			&rec.BanReason,
			&deviceSilentUntil,
			&rec.CurGold,
			&rec.TotalGold,
			&rec.CurToken,
			&rec.TotalToken,
			&rec.CurDirectInviterShare,
			&rec.TotalDirectInviterShare,
			&rec.CurSecondInviterShare,
			&rec.TotalSecondInviterShare,
			&rec.CurDownlineL1Contrib,
			&rec.TotalDownlineL1Contrib,
			&rec.CurDownlineL2Contrib,
			&rec.TotalDownlineL2Contrib,
			&rec.NotifyWatermarkDownlineL1,
			&rec.NotifyWatermarkDownlineL2,
			&notifyPendingSince,
		); err != nil {
			return err
		}
		if deviceSilentUntil.Valid {
			t := deviceSilentUntil.Time
			rec.DeviceSilentUntil = &t
		}
		if notifyPendingSince.Valid {
			t := notifyPendingSince.Time
			rec.NotifyPendingSince = &t
		}
		out = rec
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (s *MySQLPlayerRepository) FindOldestActiveGuestAccountByDeviceID(ctx context.Context, deviceID string) (*AuthUserRecord, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, nil
	}
	const q = `
SELECT
  a.account_id,
  a.account_id,
  a.account_type,
  a.account_value,
  COALESCE(a.email,''),
  COALESCE(a.phone,''),
  COALESCE(a.password_hash,''),
  COALESCE(a.display_name,''),
  COALESCE(a.provider,''),
  COALESCE(a.invited_user_id,''),
  COALESCE(a.device_id,''),
  COALESCE(a.registration_ip,''),
  COALESCE(a.ad_rewards_disabled, 0),
  COALESCE(a.status, 1),
  COALESCE(a.ban_reason, ''),
  a.device_silent_until,
  COALESCE(a.curgold, 0),
  COALESCE(a.totalgold, 0),
  COALESCE(a.curtoken, 0),
  COALESCE(a.totaltoken, 0),
  COALESCE(a.cur_direct_inviter_share, 0),
  COALESCE(a.total_direct_inviter_share, 0),
  COALESCE(a.cur_second_inviter_share, 0),
  COALESCE(a.total_second_inviter_share, 0)
FROM auth_device_account_map dm
JOIN auth_accounts a
  ON a.account_id = dm.account_id
WHERE a.deleted_at IS NULL AND dm.device_id = ? AND COALESCE(a.status,1) <> 2
AND LOWER(TRIM(a.account_type)) = 'guest'
ORDER BY dm.created_at ASC
LIMIT 1`
	var out *AuthUserRecord
	err := s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, q, deviceID)
		rec := &AuthUserRecord{}
		var deviceSilentUntil sql.NullTime
		var notifyPendingSince sql.NullTime
		if err := row.Scan(
			&rec.UserID,
			&rec.AccountID,
			&rec.AccountType,
			&rec.AccountValue,
			&rec.Email,
			&rec.Phone,
			&rec.PasswordHash,
			&rec.DisplayName,
			&rec.Provider,
			&rec.InvitedUserID,
			&rec.DeviceID,
			&rec.RegistrationIP,
			&rec.AdRewardsDisabled,
			&rec.Status,
			&rec.BanReason,
			&deviceSilentUntil,
			&rec.CurGold,
			&rec.TotalGold,
			&rec.CurToken,
			&rec.TotalToken,
			&rec.CurDirectInviterShare,
			&rec.TotalDirectInviterShare,
			&rec.CurSecondInviterShare,
			&rec.TotalSecondInviterShare,
		); err != nil {
			return err
		}
		if deviceSilentUntil.Valid {
			t := deviceSilentUntil.Time
			rec.DeviceSilentUntil = &t
		}
		if notifyPendingSince.Valid {
			t := notifyPendingSince.Time
			rec.NotifyPendingSince = &t
		}
		if notifyPendingSince.Valid {
			t := notifyPendingSince.Time
			rec.NotifyPendingSince = &t
		}
		out = rec
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (s *MySQLPlayerRepository) CountActiveAccountsByDeviceID(ctx context.Context, deviceID string) (int, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return 0, nil
	}
	const q = `
SELECT COUNT(1)
FROM auth_device_account_map dm
JOIN auth_accounts a
  ON a.account_id = dm.account_id
WHERE dm.device_id = ? AND a.deleted_at IS NULL AND COALESCE(a.status,1) <> 2`
	var n int
	err := s.withRetry(ctx, func(db *sql.DB) error {
		return db.QueryRowContext(ctx, q, deviceID).Scan(&n)
	})
	return n, err
}

func (s *MySQLPlayerRepository) CreateByAccountID(ctx context.Context, rec *AuthUserRecord) error {
	if rec == nil {
		return fmt.Errorf("record is nil")
	}
	const q = `
INSERT INTO auth_accounts (
  account_id, account_type, account_value,
  email, phone, password_hash, provider, display_name, invited_user_id,
  device_id, registration_ip, ad_rewards_disabled,
  curgold, totalgold, curtoken, totaltoken,
  cur_direct_inviter_share, total_direct_inviter_share, cur_second_inviter_share, total_second_inviter_share
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	return s.withRetry(ctx, func(db *sql.DB) error {
		adDis := rec.AdRewardsDisabled
		if adDis != 0 {
			adDis = 1
		}
		_, err := db.ExecContext(
			ctx,
			q,
			rec.AccountID,
			rec.AccountType,
			rec.AccountValue,
			rec.Email,
			rec.Phone,
			rec.PasswordHash,
			rec.Provider,
			rec.DisplayName,
			rec.InvitedUserID,
			strings.TrimSpace(rec.DeviceID),
			strings.TrimSpace(rec.RegistrationIP),
			adDis,
			rec.CurGold,
			rec.TotalGold,
			rec.CurToken,
			rec.TotalToken,
			rec.CurDirectInviterShare,
			rec.TotalDirectInviterShare,
			rec.CurSecondInviterShare,
			rec.TotalSecondInviterShare,
		)
		return err
	})
}

func (s *MySQLPlayerRepository) ResolveAccountIDByInviteCode(ctx context.Context, inviteCode string) (string, error) {
	inviteCode = strings.TrimSpace(inviteCode)
	if inviteCode == "" {
		return "", nil
	}
	const q = `SELECT account_id FROM auth_invite_codes WHERE invite_code = ? LIMIT 1`
	var out string
	err := s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, q, inviteCode)
		return row.Scan(&out)
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *MySQLPlayerRepository) AddInviteMember(ctx context.Context, inviterAccountID string, inviteeAccountID string) error {
	inviterAccountID = strings.TrimSpace(inviterAccountID)
	inviteeAccountID = strings.TrimSpace(inviteeAccountID)
	if inviterAccountID == "" || inviteeAccountID == "" {
		return fmt.Errorf("invalid invite relation")
	}
	const q = `
INSERT INTO auth_invite_members(accountid, inviteaccountid)
VALUES(?, ?)
ON DUPLICATE KEY UPDATE updated_at = NOW()`
	return s.withRetry(ctx, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, q, inviterAccountID, inviteeAccountID)
		return err
	})
}

func (s *MySQLPlayerRepository) ListInviteMembersByAccountID(ctx context.Context, accountID string) ([]InviteMemberRecord, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return []InviteMemberRecord{}, nil
	}
	const q = `
SELECT
  COALESCE(a.account_id, ''),
  COALESCE(a.nickname, ''),
  COALESCE(a.display_name, ''),
  COALESCE(a.email, ''),
  COALESCE(a.phone, ''),
  COALESCE(a.provider, ''),
  DATE_FORMAT(a.created_at, '%Y-%m-%d %H:%i:%s'),
  COALESCE(a.curgold, 0),
  COALESCE(a.totalgold, 0),
  COALESCE(a.curtoken, 0),
  COALESCE(a.totaltoken, 0),
  COALESCE(a.cur_direct_inviter_share, 0),
  COALESCE(a.total_direct_inviter_share, 0),
  COALESCE(a.cur_second_inviter_share, 0),
  COALESCE(a.total_second_inviter_share, 0)
FROM auth_invite_members m
JOIN auth_accounts a
  ON a.account_id = m.inviteaccountid AND a.deleted_at IS NULL
WHERE m.accountid = ?
ORDER BY m.created_at DESC`
	out := make([]InviteMemberRecord, 0)
	err := s.withRetry(ctx, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, q, accountID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r InviteMemberRecord
			if se := rows.Scan(
				&r.AccountID, &r.Nickname, &r.DisplayName, &r.Email, &r.Phone, &r.Provider, &r.CreatedAt,
				&r.CurGold, &r.TotalGold, &r.CurToken, &r.TotalToken,
				&r.CurDirectInviterShare, &r.TotalDirectInviterShare, &r.CurSecondInviterShare, &r.TotalSecondInviterShare,
			); se != nil {
				return se
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (s *MySQLPlayerRepository) GetInviterInfoByAccountID(ctx context.Context, accountID string) (directAccountID string, directNickname string, secondAccountID string, secondNickname string, err error) {
	const q = `
SELECT
  COALESCE(p1.account_id, ''),
  COALESCE(NULLIF(p1.nickname, ''), NULLIF(p1.display_name, ''), ''),
  COALESCE(p2.account_id, ''),
  COALESCE(NULLIF(p2.nickname, ''), NULLIF(p2.display_name, ''), '')
FROM auth_accounts cur
LEFT JOIN auth_accounts p1
  ON p1.account_id = cur.invited_user_id AND p1.deleted_at IS NULL
LEFT JOIN auth_accounts p2
  ON p2.account_id = p1.invited_user_id AND p2.deleted_at IS NULL
WHERE cur.account_id = ? AND cur.deleted_at IS NULL
LIMIT 1`
	err = s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, q, accountID)
		return row.Scan(&directAccountID, &directNickname, &secondAccountID, &secondNickname)
	})
	return
}

func (s *MySQLPlayerRepository) GetInviteCodeByAccountID(ctx context.Context, accountID string) (string, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return "", nil
	}
	const q = `SELECT invite_code FROM auth_invite_codes WHERE account_id = ? LIMIT 1`
	var out string
	err := s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, q, accountID)
		return row.Scan(&out)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (s *MySQLPlayerRepository) AllocateInviteCodeOnRegister(ctx context.Context, accountID string) (string, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return "", fmt.Errorf("account_id is empty")
	}
	const qGet = `SELECT invite_code FROM auth_invite_codes WHERE account_id = ? LIMIT 1`
	const qDeleteEmptyByAccount = `DELETE FROM auth_invite_codes WHERE account_id = ? AND TRIM(invite_code) = ''`
	const qInsert = `INSERT INTO auth_invite_codes(invite_code, account_id) VALUES(?, ?)`
	for attempt := 0; attempt < inviteCodeInsertMaxRetries; attempt++ {
		var out string
		err := s.withRetry(ctx, func(db *sql.DB) error {
			row := db.QueryRowContext(ctx, qGet, accountID)
			scanErr := row.Scan(&out)
			if scanErr == nil {
				out = strings.TrimSpace(out)
				if out != "" {
					return nil
				}
				if _, delErr := db.ExecContext(ctx, qDeleteEmptyByAccount, accountID); delErr != nil {
					return delErr
				}
				out = ""
			} else if scanErr != sql.ErrNoRows {
				return scanErr
			}
			candidate := newInviteCode()
			_, insErr := db.ExecContext(ctx, qInsert, candidate, accountID)
			if insErr != nil {
				return insErr
			}
			out = candidate
			return nil
		})
		if err == nil {
			return strings.TrimSpace(out), nil
		}
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "duplicate") || strings.Contains(msg, "1062") {
			continue
		}
		return "", err
	}
	return "", fmt.Errorf("failed to allocate invite_code after retries")
}

func (s *MySQLPlayerRepository) UpdatePasswordByAccountID(ctx context.Context, accountID string, passwordHash string) error {
	const q = `
UPDATE auth_accounts
SET password_hash = ?, provider = 'password', updated_at = NOW()
WHERE account_id = ? AND deleted_at IS NULL
LIMIT 1`
	return s.withRetry(ctx, func(db *sql.DB) error {
		res, err := db.ExecContext(ctx, q, passwordHash, accountID)
		if err != nil {
			return err
		}
		aff, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if aff <= 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *MySQLPlayerRepository) UpdateGuestVerifiedContactByAccountID(ctx context.Context, accountID, accountType, accountValue, email, phone, provider string) error {
	accountID = strings.TrimSpace(accountID)
	accountType = strings.ToLower(strings.TrimSpace(accountType))
	accountValue = strings.TrimSpace(accountValue)
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)
	provider = strings.ToLower(strings.TrimSpace(provider))
	if accountID == "" {
		return fmt.Errorf("empty account id")
	}
	if accountType == "" || accountValue == "" || provider == "" {
		return fmt.Errorf("empty verified account fields")
	}
	const q = `
UPDATE auth_accounts
SET account_type = ?, account_value = ?, email = ?, phone = ?, provider = ?, updated_at = NOW()
WHERE account_id = ? AND deleted_at IS NULL AND LOWER(TRIM(account_type)) = 'guest'
LIMIT 1`
	return s.withRetry(ctx, func(db *sql.DB) error {
		res, err := db.ExecContext(ctx, q, accountType, accountValue, email, phone, provider, accountID)
		if err != nil {
			return err
		}
		aff, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if aff <= 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *MySQLPlayerRepository) SetDeviceSilentUntilByAccountID(ctx context.Context, accountID string, silentUntil time.Time) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("empty account id")
	}
	const q = `
UPDATE auth_accounts
SET device_silent_until = ?, updated_at = NOW()
WHERE account_id = ? AND deleted_at IS NULL
LIMIT 1`
	return s.withRetry(ctx, func(db *sql.DB) error {
		res, err := db.ExecContext(ctx, q, silentUntil, accountID)
		if err != nil {
			return err
		}
		aff, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if aff <= 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *MySQLPlayerRepository) ClearDeviceSilentByAccountID(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("empty account id")
	}
	const q = `
UPDATE auth_accounts
SET device_silent_until = NULL, updated_at = NOW()
WHERE account_id = ? AND deleted_at IS NULL
LIMIT 1`
	return s.withRetry(ctx, func(db *sql.DB) error {
		res, err := db.ExecContext(ctx, q, accountID)
		if err != nil {
			return err
		}
		aff, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if aff <= 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *MySQLPlayerRepository) BanAccountByAccountID(ctx context.Context, accountID string, reason string) error {
	accountID = strings.TrimSpace(accountID)
	reason = strings.TrimSpace(reason)
	if accountID == "" {
		return fmt.Errorf("empty account id")
	}
	const q = `
UPDATE auth_accounts
SET status = 2, ban_reason = ?, device_silent_until = NULL, updated_at = NOW()
WHERE account_id = ? AND deleted_at IS NULL
LIMIT 1`
	return s.withRetry(ctx, func(db *sql.DB) error {
		res, err := db.ExecContext(ctx, q, reason, accountID)
		if err != nil {
			return err
		}
		aff, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if aff <= 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *MySQLPlayerRepository) UpdateDisplayNameByAccountID(ctx context.Context, accountID string, displayName string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("empty account id")
	}
	const q = `
UPDATE auth_accounts
SET display_name = ?, updated_at = NOW()
WHERE account_id = ? AND deleted_at IS NULL
LIMIT 1`
	return s.withRetry(ctx, func(db *sql.DB) error {
		res, err := db.ExecContext(ctx, q, displayName, accountID)
		if err != nil {
			return err
		}
		aff, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if aff <= 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ExchangeGoldForToken 消耗账号全部 curgold，按 1:1 增加 curtoken 与 totaltoken；totalgold 不变。
func (s *MySQLPlayerRepository) ExchangeGoldForToken(ctx context.Context, accountID string) (tokenDelta, curGold, curToken, totalGold, totalToken float64, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return 0, 0, 0, 0, 0, fmt.Errorf("empty account id")
	}
	err = s.withRetry(ctx, func(db *sql.DB) error {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer func() { _ = tx.Rollback() }()

		var cg, tg, cm, tm float64
		row := tx.QueryRowContext(ctx, `
SELECT COALESCE(curgold,0), COALESCE(totalgold,0), COALESCE(curtoken,0), COALESCE(totaltoken,0)
FROM auth_accounts WHERE account_id = ? AND deleted_at IS NULL FOR UPDATE`, accountID)
		if e := row.Scan(&cg, &tg, &cm, &tm); e != nil {
			return e
		}
		if cg <= 0 {
			return fmt.Errorf("no gold to exchange")
		}
		td := cg
		newCG := 0.0
		newCM := cm + td
		newTM := tm + td
		res, e := tx.ExecContext(ctx, `
UPDATE auth_accounts
SET curgold = ?, curtoken = ?, totaltoken = ?, updated_at = CURRENT_TIMESTAMP
WHERE account_id = ? AND deleted_at IS NULL`, newCG, newCM, newTM, accountID)
		if e != nil {
			return e
		}
		aff, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if aff <= 0 {
			return ErrNotFound
		}
		if e := tx.Commit(); e != nil {
			return e
		}
		tokenDelta = td
		curGold = newCG
		curToken = newCM
		totalGold = tg
		totalToken = newTM
		return nil
	})
	return tokenDelta, curGold, curToken, totalGold, totalToken, err
}

func (s *MySQLPlayerRepository) accountRowExists(ctx context.Context, db *sql.DB, accountID string) (bool, error) {
	var one int
	err := db.QueryRowContext(ctx, `
SELECT 1 FROM auth_accounts WHERE account_id = ? AND deleted_at IS NULL LIMIT 1`, accountID).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *MySQLPlayerRepository) UpdateAccountMetricsByAccountID(ctx context.Context, accountID string, curGold, totalGold, curToken, totalToken, curDirectShare, totalDirectShare, curSecondShare, totalSecondShare float64) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("empty account id")
	}
	const q = `
UPDATE auth_accounts
SET curgold = ?, totalgold = ?, curtoken = ?, totaltoken = ?,
    cur_direct_inviter_share = ?, total_direct_inviter_share = ?,
    cur_second_inviter_share = ?, total_second_inviter_share = ?,
    updated_at = NOW()
WHERE account_id = ? AND deleted_at IS NULL
LIMIT 1`
	return s.withRetry(ctx, func(db *sql.DB) error {
		res, err := db.ExecContext(ctx, q,
			curGold, totalGold, curToken, totalToken,
			curDirectShare, totalDirectShare, curSecondShare, totalSecondShare,
			accountID)
		if err != nil {
			return err
		}
		aff, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if aff > 0 {
			return nil
		}
		// MySQL may report 0 rows affected when SET values are unchanged (e.g. GM ledger SetCurGold then metrics patch).
		ok, err := s.accountRowExists(ctx, db, accountID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		return nil
	})
}

func (s *MySQLPlayerRepository) Ping(ctx context.Context) error {
	return s.withRetry(ctx, func(db *sql.DB) error {
		pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		return db.PingContext(pctx)
	})
}

func (s *MySQLPlayerRepository) withRetry(ctx context.Context, fn func(db *sql.DB) error) error {
	// fast path
	db, err := s.getHealthyDB(ctx)
	if err != nil {
		return err
	}
	runErr := fn(db)
	if !isTransientDBErr(runErr) {
		return runErr
	}

	// reconnect once and retry same request
	if err = s.reconnect(ctx); err != nil {
		return err
	}
	db, err = s.getHealthyDB(ctx)
	if err != nil {
		return err
	}
	return fn(db)
}

func (s *MySQLPlayerRepository) getHealthyDB(ctx context.Context) (*sql.DB, error) {
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	if db == nil {
		if err := s.reconnect(ctx); err != nil {
			return nil, err
		}
		s.mu.Lock()
		db = s.db
		s.mu.Unlock()
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err == nil {
		return db, nil
	}
	if err := s.reconnect(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	db = s.db
	s.mu.Unlock()
	return db, nil
}

func (s *MySQLPlayerRepository) reconnect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// close old pool first
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		db, err := sql.Open("mysql", s.dsn)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 300 * time.Millisecond)
			continue
		}
		db.SetConnMaxLifetime(30 * time.Minute)
		db.SetMaxOpenConns(20)
		db.SetMaxIdleConns(5)

		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		pingErr := db.PingContext(pingCtx)
		cancel()
		if pingErr != nil {
			lastErr = pingErr
			_ = db.Close()
			time.Sleep(time.Duration(i+1) * 300 * time.Millisecond)
			continue
		}
		s.db = db
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("mysql reconnect failed")
	}
	return fmt.Errorf("mysql reconnect failed: %w", lastErr)
}

func isTransientDBErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrConnDone) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "invalid connection") ||
		strings.Contains(msg, "no connection could be made")
}

// 9 位，数字 + 大写字母，去掉易混的 0/O、1/I（仅大写，不包含小写 l）
const inviteCodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

func newInviteCode() string {
	const n = 9
	radix := uint64(len(inviteCodeAlphabet))
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		t := uint64(time.Now().UnixNano())
		for i := 0; i < n; i++ {
			buf[i] = inviteCodeAlphabet[t%radix]
			t /= radix
		}
		return string(buf)
	}
	for i := 0; i < n; i++ {
		buf[i] = inviteCodeAlphabet[int(buf[i])%len(inviteCodeAlphabet)]
	}
	return string(buf)
}
