package api_executer

import (
	"encoding/json"
	"fmt"

	"github.com/0xdevelop/project_template_go/api/api_config"
	"github.com/george012/gtbox/gtbox_encryption"
)

func normalizeArguments(arguments interface{}, encryptionKey string) (map[string]interface{}, error) {
	if api_config.CurrentApiCfg == nil || !api_config.CurrentApiCfg.NeedEncryption {
		params, paramsOK := arguments.(map[string]interface{})
		if !paramsOK {
			return nil, invalidArgumentsError(
				"arguments must be an object when need_encryption is false",
			)
		}
		return params, nil
	}

	encrypted, encryptedOK := arguments.(string)
	if !encryptedOK || encrypted == "" {
		return nil, invalidArgumentsError(
			"arguments must be an encrypted string when need_encryption is true",
		)
	}

	decrypted := gtbox_encryption.GTDec(encrypted, encryptionKey)
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(decrypted), &params); err != nil {
		return nil, invalidArgumentsError(fmt.Sprintf(
			"arguments decrypt result must be a JSON object: %v",
			err,
		))
	}
	return params, nil
}

func invalidArgumentsError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidArguments, message)
}
