package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (s *MySQLPlayerRepository) CountAccountsRegisteredTodayByDeviceID(ctx context.Context, deviceID string) (n int, err error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return 0, fmt.Errorf("device id is empty")
	}
	const q = `
SELECT COUNT(*) FROM auth_accounts
WHERE deleted_at IS NULL AND device_id = ? AND DATE(created_at) = CURDATE()`
	err = s.withRetry(ctx, func(db *sql.DB) error {
		return db.QueryRowContext(ctx, q, deviceID).Scan(&n)
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *MySQLPlayerRepository) CountAccountsRegisteredTodayByRegistrationIP(ctx context.Context, registrationIP string) (n int, err error) {
	registrationIP = strings.TrimSpace(registrationIP)
	if registrationIP == "" {
		return 0, fmt.Errorf("registration ip is empty")
	}
	const q = `
SELECT COUNT(*) FROM auth_accounts
WHERE deleted_at IS NULL AND registration_ip = ? AND DATE(created_at) = CURDATE()`
	err = s.withRetry(ctx, func(db *sql.DB) error {
		return db.QueryRowContext(ctx, q, registrationIP).Scan(&n)
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *MySQLPlayerRepository) UpsertDeviceAccountMap(ctx context.Context, deviceID, accountID string) error {
	deviceID = strings.TrimSpace(deviceID)
	accountID = strings.TrimSpace(accountID)
	if deviceID == "" || accountID == "" {
		return nil
	}
	const q = `
INSERT INTO auth_device_account_map (device_id, account_id) VALUES (?, ?)
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP`
	return s.withRetry(ctx, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, q, deviceID, accountID)
		return err
	})
}
