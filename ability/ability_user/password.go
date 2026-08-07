package ability_user

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Time       uint32 = 2
	argon2Memory     uint32 = 19 * 1024
	argon2Threads    uint8  = 1
	argon2SaltLength        = 16
	argon2KeyLength  uint32 = 32
)

func ValidatePassword(value string) error {
	length := utf8.RuneCountInString(value)
	if length < 15 || length > 128 || len(value) > 1024 {
		return errors.New("invalid password")
	}
	return nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLength,
	)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory,
		argon2Time,
		argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func burnPasswordVerificationCost(password string) {
	_ = argon2.IDKey(
		[]byte(password),
		[]byte("template-auth-dummy"),
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLength,
	)
}

func verifyPassword(encoded string, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, errors.New("invalid password hash")
	}

	var memory uint64
	var iterations uint64
	var threads uint64
	for _, parameter := range strings.Split(parts[3], ",") {
		keyValue := strings.SplitN(parameter, "=", 2)
		if len(keyValue) != 2 {
			return false, errors.New("invalid password hash parameters")
		}
		value, err := strconv.ParseUint(keyValue[1], 10, 32)
		if err != nil {
			return false, errors.New("invalid password hash parameters")
		}
		switch keyValue[0] {
		case "m":
			memory = value
		case "t":
			iterations = value
		case "p":
			threads = value
		default:
			return false, errors.New("invalid password hash parameters")
		}
	}
	if memory < 8*1024 || memory > 256*1024 ||
		iterations < 1 || iterations > 10 ||
		threads < 1 || threads > 8 {
		return false, errors.New("unsafe password hash parameters")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false, errors.New("invalid password hash salt")
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return false, errors.New("invalid password hash value")
	}
	actual := argon2.IDKey(
		[]byte(password),
		salt,
		uint32(iterations),
		uint32(memory),
		uint8(threads),
		uint32(len(expected)),
	)
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}
