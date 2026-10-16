package mysql

import (
	"context"
	"database/sql"
	"strings"

	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
)

// LoginAttemptRecorder appends rows to login_attempts (migration 0001).
type LoginAttemptRecorder struct {
	db *sql.DB
}

func NewLoginAttemptRecorder(db *sql.DB) *LoginAttemptRecorder {
	return &LoginAttemptRecorder{db: db}
}

func (r *LoginAttemptRecorder) Record(ctx context.Context, rec iamapp.LoginAttemptRecord) error {
	loginID := strings.TrimSpace(rec.LoginID)
	if loginID == "" {
		return nil
	}
	var uid interface{}
	if rec.UserID != "" {
		uid = rec.UserID
	}
	var fc interface{}
	if !rec.Success && rec.FailureCode != "" {
		fc = rec.FailureCode
	}
	success := 0
	if rec.Success {
		success = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO login_attempts (login_id, user_id, success, failure_code, ip, user_agent)
		VALUES (?, ?, ?, ?, ?, ?)
	`, truncStr(loginID, 191), uid, success, fc, truncStr(rec.IP, 64), truncStr(rec.UserAgent, 512))
	return err
}

// truncStr makes s valid UTF-8 and cuts it to max runes (the VARCHAR length). Failed attempts carry
// whatever was typed and any User-Agent: an over-long value, an invalid byte or a rune cut in half
// makes the utf8mb4 insert fail, and the attempt goes unrecorded.
func truncStr(s string, max int) string {
	s = strings.ToValidUTF8(s, "�")
	if runes := []rune(s); len(runes) > max {
		return string(runes[:max])
	}
	return s
}
