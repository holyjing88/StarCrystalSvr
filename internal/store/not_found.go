package store

import (
	"database/sql"
	"errors"
)

// ErrNotFound 表示查找单条玩家/账号记录未果。SQL 实现的 Repository 可把 sql.ErrNoRows 映射为 ErrNotFound，
// 也可以直接返回 ErrNotFound；Service 层只应使用 errors.Is(..., ErrNotFound) 或 IsNotFound，禁止 import database/sql。
var ErrNotFound = errors.New("store: not found")

// IsNotFound 判断 err 是否为「记录不存在」（含 sql.ErrNoRows）。Service / API 层请用本函数，不要直接比对 sql.ErrNoRows。
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows)
}
