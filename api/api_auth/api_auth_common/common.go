package api_auth_common

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/mail"
	"regexp"
	"strings"

	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/george012/gtbox/gtbox_log"
)

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
var VerificationCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

func InputObject(input interface{}) (map[string]interface{}, bool) {
	value, ok := input.(map[string]interface{})
	return value, ok
}

func RequiredString(input map[string]interface{}, name string) (string, bool) {
	value, ok := input[name].(string)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func RequiredRawString(input map[string]interface{}, name string) (string, bool) {
	value, ok := input[name].(string)
	return value, ok && value != ""
}

func HasOnlyKeys(input map[string]interface{}, allowed ...string) bool {
	if len(input) != len(allowed) {
		return false
	}
	allowedKeys := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedKeys[key] = struct{}{}
	}
	for key := range input {
		if _, ok := allowedKeys[key]; !ok {
			return false
		}
	}
	return true
}

func NormalizeEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 320 {
		return "", api_error_code.ErrInvalidArguments
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return "", api_error_code.ErrInvalidArguments
	}
	at := strings.LastIndexByte(value, '@')
	if at < 1 || at == len(value)-1 {
		return "", api_error_code.ErrInvalidArguments
	}
	return strings.ToLower(value), nil
}

func NormalizePhone(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !phonePattern.MatchString(value) {
		return "", api_error_code.ErrInvalidArguments
	}
	return value, nil
}

func LogInternalFailure(operation string) {
	gtbox_log.LogErrorf("auth operation failed: %s", operation)
}

func NewVerificationCode() (string, error) {
	number, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", number.Int64()), nil
}

func NewRandomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func VerificationCodeHash(secret, recipient, code string) string {
	digest := hmac.New(sha256.New, []byte(secret))
	_, _ = digest.Write([]byte(recipient))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(code))
	return hex.EncodeToString(digest.Sum(nil))
}

func SecureStringEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func TokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func InputSchema(
	properties map[string]interface{},
	required ...string,
) map[string]interface{} {
	schema := map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	// 零必填方法省略 required 键，避免序列化出 required: null 的畸形 schema。
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func StringSchema(
	format string,
	minLength int,
	maxLength int,
) map[string]interface{} {
	schema := map[string]interface{}{
		"type":      "string",
		"minLength": minLength,
		"maxLength": maxLength,
	}
	if format != "" {
		schema["format"] = format
	}
	return schema
}

func EnumStringSchema(values ...string) map[string]interface{} {
	return map[string]interface{}{
		"type": "string",
		"enum": values,
	}
}

func PhoneSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":    "string",
		"pattern": `^\+[1-9][0-9]{7,14}$`,
	}
}

func VerificationCodeSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":    "string",
		"pattern": "^[0-9]{6}$",
	}
}

func TokenInputSchema(name string) map[string]interface{} {
	return InputSchema(
		map[string]interface{}{
			name: StringSchema("", 1, 8192),
		},
		name,
	)
}
