package authbiz

import "auralogic/internal/pkg/bizerr"

func InvalidEmailOrPassword() *bizerr.Error {
	return bizerr.New("auth.invalidEmailOrPassword", "Invalid email or password")
}

func AccountDisabled() *bizerr.Error {
	return bizerr.New("auth.accountDisabled", "User account has been disabled")
}

func PasswordLoginDisabled() *bizerr.Error {
	return bizerr.New("auth.passwordLoginDisabled", "Password login is disabled, please use quick login or OAuth login")
}

func EmailNotVerified() *bizerr.Error {
	return bizerr.New("auth.emailNotVerified", "Please verify your email before logging in")
}

func EmailAlreadyInUse() *bizerr.Error {
	return bizerr.New("auth.emailAlreadyInUse", "Email already in use")
}

func PhoneAlreadyInUse() *bizerr.Error {
	return bizerr.New("auth.phoneAlreadyInUse", "Phone number already in use")
}

func IncorrectOldPassword() *bizerr.Error {
	return bizerr.New("auth.incorrectOldPassword", "Incorrect old password")
}

func UserNotFound() *bizerr.Error {
	return bizerr.New("auth.userNotFound", "User not found")
}

func ResetTokenExpired() *bizerr.Error {
	return bizerr.New("auth.resetTokenExpired", "Reset token expired or invalid")
}

func CodeExpired() *bizerr.Error {
	return bizerr.New("auth.codeExpired", "Verification code expired or invalid")
}

func InvalidCode() *bizerr.Error {
	return bizerr.New("auth.invalidCode", "Invalid verification code")
}

func RegistrationDisabled() *bizerr.Error {
	return bizerr.New("auth.registrationDisabled", "Registration is disabled")
}

func EmailLoginUnavailable() *bizerr.Error {
	return bizerr.New("auth.emailLoginUnavailable", "Email login is not available")
}

func EmailLoginDisabled() *bizerr.Error {
	return bizerr.New("auth.emailLoginDisabled", "Email login is disabled")
}

func PasswordResetDisabled() *bizerr.Error {
	return bizerr.New("auth.passwordResetDisabled", "Password reset is disabled")
}

func SMSServiceUnavailable() *bizerr.Error {
	return bizerr.New("auth.smsServiceUnavailable", "SMS service is not available")
}

func PhoneLoginDisabled() *bizerr.Error {
	return bizerr.New("auth.phoneLoginDisabled", "Phone login is disabled")
}

func PhoneRegistrationDisabled() *bizerr.Error {
	return bizerr.New("auth.phoneRegistrationDisabled", "Phone registration is disabled")
}

func PhonePasswordResetDisabled() *bizerr.Error {
	return bizerr.New("auth.phonePasswordResetDisabled", "Phone password reset is disabled")
}

func InvalidPhoneFormat() *bizerr.Error {
	return bizerr.New("auth.invalidPhoneFormat", "Invalid phone number format")
}

func CaptchaRequired() *bizerr.Error {
	return bizerr.New("auth.captchaRequired", "Captcha is required")
}

func CaptchaFailed() *bizerr.Error {
	return bizerr.New("auth.captchaFailed", "Captcha verification failed")
}
