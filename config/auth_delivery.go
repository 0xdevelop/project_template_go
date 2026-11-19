package config

import (
	"bytes"
	"errors"
	"strings"

	"github.com/goccy/go-json"
	"github.com/goccy/go-yaml"
	"github.com/pelletier/go-toml/v2"
)

// validateAuthDeliveryDocument rejects misspelled/legacy provider blocks instead of silently treating a misspelled email configuration as absent.
// This is limited to auth routing/email/verification keys; no credentials or configuration values enter errors.
func validateAuthDeliveryDocument(buf []byte, ext string) error {
	var document map[string]interface{}
	var err error
	switch ext {
	case ".json":
		decoder := json.NewDecoder(bytes.NewReader(buf))
		decoder.UseNumber()
		err = decoder.Decode(&document)
	case ".toml":
		err = toml.Unmarshal(buf, &document)
	default:
		err = yaml.Unmarshal(buf, &document)
		if err != nil && ext != ".yaml" && ext != ".yml" {
			err = toml.Unmarshal(buf, &document)
		}
	}
	if err != nil {
		return errors.New("cannot inspect auth delivery configuration")
	}
	raw, exists := document["auth_cfg"]
	if !exists || raw == nil {
		return nil
	}
	auth, ok := raw.(map[string]interface{})
	if !ok {
		return errors.New("auth_cfg must be an object")
	}
	if err = rejectUnknownAuthKeys(auth, "auth_cfg", "enabled auth_type email verification session bridge"); err != nil {
		return err
	}
	if value, present := auth["enabled"]; present {
		if _, ok := value.(bool); !ok {
			return errors.New("auth_cfg.enabled must be true or false")
		}
	}

	for _, block := range []struct{ name, allowed string }{
		{"email", "enabled provider api_key from reply_to product_name verification_subject"},
		{"verification", "code_ttl_seconds max_attempts send_interval_seconds resend_interval_seconds hourly_send_limit code_hash_secret"},
	} {
		raw, present := auth[block.name]
		if !present {
			continue
		}
		object, ok := raw.(map[string]interface{})
		if !ok {
			return errors.New("auth_cfg." + block.name + " must be an object")
		}
		if err = rejectUnknownAuthKeys(object, "auth_cfg."+block.name, block.allowed); err != nil {
			return err
		}
		if block.name == "email" {
			for _, key := range []string{"provider", "api_key", "from", "reply_to", "product_name", "verification_subject"} {
				if value, present := object[key]; present {
					if _, ok := value.(string); !ok {
						return errors.New("auth_cfg.email." + key + " must be a string")
					}
				}
			}
			if value, present := object["enabled"]; present {
				if _, ok := value.(bool); !ok {
					return errors.New("auth_cfg.email.enabled must be true or false")
				}
			}
		}
		if block.name == "verification" {
			for _, key := range []string{"code_ttl_seconds", "max_attempts", "send_interval_seconds", "resend_interval_seconds", "hourly_send_limit"} {
				if value, present := object[key]; present {
					switch number := value.(type) {
					case int:
						if number < 1 {
							return errors.New("auth_cfg.verification." + key + " must be a positive integer")
						}
					case int64:
						if number < 1 {
							return errors.New("auth_cfg.verification." + key + " must be a positive integer")
						}
					case uint64:
						if number < 1 {
							return errors.New("auth_cfg.verification." + key + " must be a positive integer")
						}
					case json.Number:
						// Preserve JSON number spelling: fractions/exponents are not silently converted to integer settings.
						integer, err := number.Int64()
						if err != nil || integer < 1 {
							return errors.New("auth_cfg.verification." + key + " must be a positive integer")
						}

					default:
						return errors.New("auth_cfg.verification." + key + " must be a positive integer")
					}
				}
			}
			if value, present := object["code_hash_secret"]; present {
				if _, ok := value.(string); !ok {
					return errors.New("auth_cfg.verification.code_hash_secret must be a string")
				}
			}
		}
	}
	return nil
}
func rejectUnknownAuthKeys(object map[string]interface{}, path, names string) error {
	allowed := map[string]bool{}
	for _, name := range strings.Fields(names) {
		allowed[name] = true
	}
	for name := range object {
		if !allowed[name] {
			return errors.New(path + " contains an unknown key; use generic email/verification configuration")
		}
	}
	return nil
}
