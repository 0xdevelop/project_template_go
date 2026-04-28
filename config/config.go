package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/0xYeah/project_template_go/api/api_config"
	"github.com/0xYeah/project_template_go/api/api_json_rpc/json_rpc_permissions"
	"github.com/george012/gtbox/gtbox_log"
	"github.com/goccy/go-json"
	"github.com/goccy/go-yaml"
	"github.com/pelletier/go-toml/v2"
)

const (
	ProjectName     = "project_template_go"
	ProjectVersion  = "v0.0.8"
	ProjectBundleID = "com.project_template_go.project_template_go"
	apiPortDefault  = 12095
)

var (
	GlobalConfig *FileConfig
	HardSN       string
)

type FileConfig struct {
	ApiCfg *api_config.ApiConfig `toml:"api_cfg" yaml:"api_cfg" json:"api_cfg" comment:"API configurations"`
	Auth   Auth                  `toml:"auth"    yaml:"auth"    json:"auth"`
}

type Auth struct {
	Username string `toml:"username" yaml:"username" json:"username"`
	Password string `toml:"password" yaml:"password" json:"password"`
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
				nested = reflect.New(field.Type.Elem()).Interface()
			} else {
				nested = fieldVal.Interface()
			}
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
	switch ext {
	case ".yaml", ".yml":
		return yaml.Unmarshal(buf, GlobalConfig) // 直接解析到已初始化的结构
	case ".json":
		return json.Unmarshal(buf, GlobalConfig)
	case ".toml":
		return toml.Unmarshal(buf, GlobalConfig)
	default:
		if err := yaml.Unmarshal(buf, GlobalConfig); err != nil {
			if err2 := toml.Unmarshal(buf, GlobalConfig); err2 != nil {
				return json.Unmarshal(buf, GlobalConfig)
			}
		}
		return nil
	}
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

	err = os.WriteFile(file, buf, 0755)
	if err != nil {
		return errors.New(fmt.Sprintf("无法写入配置文件 [%s]: %s", file, err.Error()))
	}

	return nil
}

func generateDefaultConfig() *FileConfig {
	aApiPort := apiPortDefault
	clientTimeoutString := "10s"
	fileCfg := &FileConfig{
		ApiCfg: &api_config.ApiConfig{
			Enabled: true,
			Port:    aApiPort,
			Apis: []*api_config.ApiProxy{
				&api_config.ApiProxy{
					ApiPath:     "test_rpc",
					Enabled:     true,
					Address:     "http://127.0.0.1:9332",
					AuthEnabled: true,
					User:        "testuser",
					Pwd:         "testuserpwd",
				},
				&api_config.ApiProxy{
					ApiPath:     "test_rpc_02",
					Enabled:     true,
					Address:     "http://127.0.0.1:8332",
					AuthEnabled: true,
					User:        "testuser",
					Pwd:         "testuserpwd",
				},
			},
			ClientTimeout: clientTimeoutString,
			Permissions: map[string]*json_rpc_permissions.Permission{
				json_rpc_permissions.APIPermissionTagDefault: &json_rpc_permissions.Permission{
					PermissionTag:     json_rpc_permissions.APIPermissionTagDefault,
					EncryptionEnabled: true,
					AllowedUAs:        []string{},
					AllowedMethods: []string{
						"test",
					},
				},
				"test_a": &json_rpc_permissions.Permission{
					PermissionTag:     "test_a",
					EncryptionEnabled: false,
					AllowedUAs:        []string{},
					AllowedMethods: []string{
						"test_a_method_01",
						"test_a_method_02",
						"test_a_method_03",
					},
				},
				"test_b": &json_rpc_permissions.Permission{
					PermissionTag:     "test_b",
					EncryptionEnabled: true,
					AllowedUAs:        []string{},
					AllowedMethods: []string{
						"test_b_method_01",
						"test_b_method_02",
						"test_b_method_03",
					},
				},
				"test_c": &json_rpc_permissions.Permission{
					PermissionTag:     "test_c",
					EncryptionEnabled: true,
					AllowedUAs:        []string{},
					AllowedMethods:    []string{},
				},
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
