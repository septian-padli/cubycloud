package auth

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type UserResponse struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	QuotaBytes int64  `json:"quota_bytes"`
	UsedBytes  int64  `json:"used_bytes"`
}

type LoginResponse struct {
	User    UserResponse `json:"user"`
	Message string       `json:"message"`
}

type ResetPasswordRequest struct {
	Username    string `json:"username"`
	RecoveryKey string `json:"recovery_key"`
	NewPassword string `json:"new_password"`
}

type ResetPasswordResponse struct {
	Message        string `json:"message"`
	NewRecoveryKey string `json:"new_recovery_key"`
}
