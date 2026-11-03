package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/0xdevelop/project_template_go/api/api_auth/api_auth_config"
	"github.com/0xdevelop/project_template_go/frontend/frontend_config"
	"github.com/0xdevelop/project_template_go/policy/policy_config"

	"github.com/0xdevelop/project_template_go/api/api_config"
	"github.com/0xdevelop/project_template_go/api/api_grpc/api_config_grpc"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_config_jsonRPC"
	"github.com/0xdevelop/project_template_go/api/api_mcp/api_config_mcp"
	"github.com/0xdevelop/project_template_go/api/api_websocket/api_config_websocket"
	"github.com/0xdevelop/project_template_go/storage/storage_config"
	"github.com/george012/gtbox/gtbox_log"
	"github.com/goccy/go-json"
	"github.com/goccy/go-yaml"
	"github.com/pelletier/go-toml/v2"
)

const (
	ProjectName     = "project_template_go"
	ProjectVersion  = "v0.0.15"
	ProjectBundleID = "com.project_template_go.project_template_go"
	apiPortDefault  = 7095
)

var (
	GlobalConfig *FileConfig
	HardSN       string
)

type TestUIConfig struct {
	Enabled     bool   `yaml:"enabled" json:"enabled" toml:"enabled"`
	BindAddress string `yaml:"bind_address" json:"bind_address" toml:"bind_address" comment:"监听地址（必填）：127.0.0.1 仅本机；0.0.0.0 对外"`
	Port        int    `yaml:"port" json:"port" toml:"port"`
}

type FileConfig struct {
	StorageCfg  *storage_config.StorageConfig     `yaml:"storage_cfg" json:"storage_cfg" toml:"storage_cfg" comment:"存储"`
	LegacyMysql *storage_config.LegacyMysqlConfig `yaml:"mysql_cfg" json:"mysql_cfg" toml:"mysql_cfg"`
	ApiCfg      *api_config.ApiConfig             `toml:"api_cfg" yaml:"api_cfg" json:"api_cfg" comment:"API configurations"`
	FrontendCfg *frontend_config.FrontendConfig   `yaml:"frontend_cfg" json:"frontend_cfg" toml:"frontend_cfg" comment:"前端页面服务配置"`
	TestUICfg   *TestUIConfig                     `yaml:"test_ui_cfg" json:"test_ui_cfg" toml:"test_ui_cfg" comment:"开发测试台"`
	AuthCfg     *api_auth_config.AuthConfig       `yaml:"auth_cfg" json:"auth_cfg" toml:"auth_cfg" comment:"Auth configurations"`
	PolicyCfg   *policy_config.PolicyConfig       `yaml:"policy_cfg" json:"policy_cfg" toml:"policy_cfg" comment:"Policy scheduling configurations"`
}

func buildYAMLCommentMap(cfg interface{}, parentPath string) yaml.CommentMap {
	commentMap := yaml.CommentMap{}
	val := reflect.ValueOf(cfg)

	// 处理指针类型
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			if parentPath != "" { // 只处理嵌套的nil指针
				val = reflect.New(val.Type().Elem())
			} else {
				return commentMap
			}
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return commentMap
	}

	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if !field.IsExported() {
			continue
		}

		// 解析yaml tag
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "-" {
			continue
		}
		if yamlTag == "" {
			yamlTag = strings.ToLower(field.Name)
		} else if commaIdx := strings.Index(yamlTag, ","); commaIdx != -1 {
			yamlTag = yamlTag[:commaIdx]
		}

		// 修复路径：确保以$开头
		currentPath := parentPath
		if currentPath == "" {
			currentPath = "$." + yamlTag // 根路径
		} else {
			currentPath += "." + yamlTag // 嵌套路径
		}

		// 处理注释
		if comment, ok := field.Tag.Lookup("comment"); ok && comment != "" {
			commentLines := strings.Split(comment, "\n")
			comments := make([]string, 0, len(commentLines))
			for _, line := range commentLines {
				if line != "" {
					comments = append(comments, " "+line)
				}
			}
			commentMap[currentPath] = []*yaml.Comment{
				{
					Texts:    comments,
					Position: yaml.CommentLinePosition,
				},
			}
		}

		// 递归处理嵌套字段
		fieldVal := val.Field(i)
		if !fieldVal.CanInterface() {
			continue
		}

		var nested interface{}
		switch field.Type.Kind() {
		case reflect.Ptr:
			if fieldVal.IsNil() {
				continue
			}
			nested = fieldVal.Interface()
		case reflect.Struct:
			nested = fieldVal.Interface()
		case reflect.Slice:
			if fieldVal.Len() > 0 {
				nested = fieldVal.Index(0).Interface()
			} else if field.Type.Elem().Kind() == reflect.Ptr {
				nested = reflect.New(field.Type.Elem().Elem()).Interface()
			} else {
				nested = reflect.New(field.Type.Elem()).Interface()
			}
		case reflect.Map:
			if fieldVal.Len() > 0 {
				iter := fieldVal.MapRange()
				iter.Next()
				nested = iter.Value().Interface()
			} else {
				nested = reflect.New(field.Type.Elem()).Interface()
			}
		default:
			continue
		}

		nestedComments := buildYAMLCommentMap(nested, currentPath)
		for k, v := range nestedComments {
			commentMap[k] = v
		}
	}
	return commentMap
}

func LoadConfig(file string) error {
	// 确保GlobalConfig已初始化
	if GlobalConfig == nil {
		GlobalConfig = &FileConfig{}
	}

	buf, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(file))
	var decodeErr error
	switch ext {
	case ".yaml", ".yml":
		decodeErr = yaml.Unmarshal(buf, GlobalConfig)
	case ".json":
		decodeErr = json.Unmarshal(buf, GlobalConfig)
	case ".toml":
		decodeErr = toml.Unmarshal(buf, GlobalConfig)
	default:
		if decodeErr = yaml.Unmarshal(buf, GlobalConfig); decodeErr != nil {
			if decodeErr = toml.Unmarshal(buf, GlobalConfig); decodeErr != nil {
				decodeErr = json.Unmarshal(buf, GlobalConfig)
			}
		}
	}
	if decodeErr != nil {
		return decodeErr
	}
	GlobalConfig.StorageCfg, err = storage_config.Apply(GlobalConfig.StorageCfg, GlobalConfig.LegacyMysql)
	if err != nil {
		return err
	}
	if err := GlobalConfig.FrontendCfg.Validate(); err != nil {
		return err
	}
	api_auth_config.CurrentCfgAuth = GlobalConfig.AuthCfg
	policy_config.CurrentCfgPolicy = GlobalConfig.PolicyCfg
	return nil
}

func SaveConfig(file string, content *FileConfig) error {
	if file == "" {
		file = CurrentApp.AppConfigFilePath
	}
	// 写入默认配置文件内容
	var err error
	var buf []byte
	// 根据文件扩展名决定使用哪种解析方式
	ext := strings.ToLower(filepath.Ext(file))
	switch ext {
	case ".yaml", ".yml":
		buf, err = yaml.MarshalWithOptions(content, yaml.WithComment(buildYAMLCommentMap(content, "")))
	case ".json":
		buf, err = json.MarshalIndent(content, "", "    ")
	case ".toml":
		buf, err = toml.Marshal(content)
	default:
		// 未知扩展名 → 优先 yaml，回退 toml，最后 json
		buf, err = yaml.MarshalWithOptions(content, yaml.WithComment(buildYAMLCommentMap(content, "")))
		if err != nil {
			if buf2, err2 := toml.Marshal(content); err2 == nil {
				buf, err = buf2, nil
			} else {
				buf, err = json.MarshalIndent(content, "", "    ")
			}
		}
	}

	if err != nil {
		return fmt.Errorf("无法编码配置文件 [%s]: %w", file, err)
	}
	err = os.WriteFile(file, buf, 0755)
	if err != nil {
		return errors.New(fmt.Sprintf("无法写入配置文件 [%s]: %s", file, err.Error()))
	}

	return nil
}

func generateDefaultConfig() *FileConfig {
	authGateEnabledDefault := true
	aport := 13001
	// 生成的缺省配置只监听本机；对外部署由运维改成 0.0.0.0，不在代码里兜底。
	bindLocal := "127.0.0.1"
	fileCfg := &FileConfig{
		StorageCfg: storage_config.Default("./data/project_template_go.db"),
		ApiCfg: &api_config.ApiConfig{
			APICfgJsonRPC: &api_config_jsonRPC.APIConfigJsonRPC{
				Enabled:           true,
				BindAddress:       bindLocal,
				Port:              aport,
				EncryptionEnabled: false,
			},
			APICfgMCP: &api_config_mcp.APIConfigMCP{
				Enabled:          true,
				BindAddress:      bindLocal,
				Port:             aport + 1,
				MCPTransportType: api_config_mcp.MCPTransportTypeStreamableHTTP,
			},
			APICfgWebSocket: &api_config_websocket.APIConfigWebSocket{
				Enabled:     true,
				BindAddress: bindLocal,
				Port:        aport + 3,
				AllowedOrigins: []string{
					"127.0.0.1:*",
					"localhost:*",
				},
			},
			APICfgGRPC: &api_config_grpc.APIConfigGRPC{
				Enabled:     true,
				BindAddress: bindLocal,
				Port:        aport + 4,
			},
		},
		FrontendCfg: &frontend_config.FrontendConfig{
			Enabled:     true,
			BindAddress: bindLocal,
			Port:        aport + 5,
		},
		TestUICfg: &TestUIConfig{
			Enabled:     true,
			BindAddress: bindLocal,
			Port:        aport + 2,
		},
		PolicyCfg: &policy_config.PolicyConfig{
			PolicyDuration: "10s",
			WorkersScaller: 1,
		},
		AuthCfg: &api_auth_config.AuthConfig{
			Enabled:  &authGateEnabledDefault,
			AuthType: api_auth_config.AuthTypeJWT,
			Email: &api_auth_config.EmailConfig{
				Provider:            api_auth_config.EmailProviderResend,
				ProductName:         ProjectName,
				VerificationSubject: ProjectName + " verification code",
			},
			Verification: &api_auth_config.VerificationConfig{
				CodeTTLSeconds:        10 * 60,
				MaxAttempts:           5,
				ResendIntervalSeconds: 60,
				HourlySendLimit:       5,
			},
			Session: &api_auth_config.SessionConfig{
				Issuer:                 ProjectName,
				Audience:               ProjectName,
				AccessTokenTTLSeconds:  15 * 60,
				RefreshTokenTTLSeconds: 30 * 24 * 60 * 60,
			},
		},
	}
	return fileCfg
}

func SyncConfigFile(firstRunEnd func(error)) {

	if CurrentApp == nil {
		firstRunEnd(errors.New("App Not Setup "))
		return
	}

	gtbox_log.LogInfof("加载配置文件 [%s]", CurrentApp.AppConfigFilePath)
	_, err := os.Stat(CurrentApp.AppConfigFilePath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		// 获取配置文件的父目录路径
		dir := filepath.Dir(CurrentApp.AppConfigFilePath)

		// 检查父目录是否存在
		if _, err = os.Stat(dir); errors.Is(err, os.ErrNotExist) {
			// 创建父目录
			if err = os.MkdirAll(dir, 0755); err != nil {
				firstRunEnd(errors.New(fmt.Sprintf("无法创建目录 [%s]: %s", dir, err.Error())))
				return
			}
		}

		// 写入默认配置文件内容
		err = SaveConfig(CurrentApp.AppConfigFilePath, generateDefaultConfig())
		if err != nil {
			firstRunEnd(err)
			return
		}
	} else {
		buf, err := os.ReadFile(CurrentApp.AppConfigFilePath)
		if err != nil {
			firstRunEnd(errors.New(fmt.Sprintf("读取配置文件 [%s] 错误: %s", CurrentApp.AppConfigFilePath, err.Error())))
			return
		}
		if len(buf) == 0 {
			gtbox_log.LogErrorf("配置文件重置")
			err = SaveConfig(CurrentApp.AppConfigFilePath, generateDefaultConfig())
			if err != nil {
				firstRunEnd(err)
				return
			}
		}
	}

	err = LoadConfig(CurrentApp.AppConfigFilePath)

	if err != nil {
		firstRunEnd(errors.New(fmt.Sprintf("无法加载配置文件 [%s]: %s", CurrentApp.AppConfigFilePath, err.Error())))
		return
	}

}
