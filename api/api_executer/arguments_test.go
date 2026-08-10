package api_executer

import (
	"testing"

	"github.com/0xdevelop/project_template_go/api/api_config"
	"github.com/george012/gtbox/gtbox_encryption"
)

func TestNormalizePlainArguments(t *testing.T) {
	api_config.CurrentApiCfg = &api_config.ApiConfig{}

	arguments := map[string]interface{}{
		"username": "test",
	}
	params, err := normalizeArguments(arguments, "")
	if err != nil {
		t.Fatalf("normalize arguments: %v", err)
	}
	if params["username"] != "test" {
		t.Fatalf("unexpected params: %#v", params)
	}
}

func TestNormalizeEncryptedArguments(t *testing.T) {
	api_config.CurrentApiCfg = &api_config.ApiConfig{
		NeedEncryption: true,
	}
	encryptionKey := "test-client/1753421234567A9K2"
	encrypted := gtbox_encryption.GTEnc(
		`{"username":"test","password":"123456"}`,
		encryptionKey,
	)

	params, err := normalizeArguments(encrypted, encryptionKey)
	if err != nil {
		t.Fatalf("normalize encrypted arguments: %v", err)
	}
	if params["username"] != "test" || params["password"] != "123456" {
		t.Fatalf("unexpected params: %#v", params)
	}
}
