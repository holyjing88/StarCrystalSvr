package errx

import "errors"

// 账号密码登录的失败原因（客户端可按 Code 分支提示，文案与后端一致）。
var (
	ErrAccountNotFound          = errors.New("该邮箱或手机号未注册")
	// ErrPasswordResetAccountNotFound 重置密码：账号须已注册（勿通过重置流程凭空建号）。
	ErrPasswordResetAccountNotFound = errors.New("该邮箱未注册，无法重置密码，请先完成注册")
	ErrWrongPassword            = errors.New("密码错误")
	ErrPasswordLoginUnavailable = errors.New("该账号未设置密码登录，请使用验证码注册或三方登录")
	ErrInvalidEmailFormat       = errors.New("邮箱格式不正确")
	ErrInvalidPhoneFormat       = errors.New("手机号格式不正确")
)
