// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是 Timmypanel 的全部可配置项。首次启动时若配置文件不存在，
// 会用默认值生成一份（含随机密钥和初始管理员密码）。
type Config struct {
	Server struct {
		// 监听地址。公网部署时保持 127.0.0.1，由 Caddy/Nginx 反代。
		Listen string `yaml:"listen"`
		// 是否在 Cookie 上打 Secure 标记。跑 HTTPS 时必须为 true。
		Secure bool `yaml:"secure"`
		// 反代信任的来源，留空表示不信任任何代理头。
		TrustedProxies []string `yaml:"trusted_proxies"`
	} `yaml:"server"`

	Data struct {
		// 数据目录，存放 sqlite 库、上传文件和自动备份。
		Dir string `yaml:"dir"`
	} `yaml:"data"`

	Auth struct {
		// 预留的服务端密钥，首次启动自动生成。
		// 注意：当前版本没有任何代码读它 —— 会话 token 是直接取的随机数，
		// 不从这里派生。留着是给以后需要密钥的功能（比如 TOTP）用的。
		Secret string `yaml:"secret"`
		// 勾选“记住我”后的会话有效期（天）。
		RememberDays int `yaml:"remember_days"`
		// 初始管理员，仅在数据库中没有任何用户时生效。
		InitialAdmin struct {
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		} `yaml:"initial_admin"`
		// 预留：开放注册。当前版本没有注册接口，改成 true 也不会出现入口；
		// 账号仍由管理员在后台创建。公网请保持 false。
		AllowRegister bool `yaml:"allow_register"`
	} `yaml:"auth"`

	Fetch struct {
		// 是否允许抓取内网地址的图标/标题。默认关闭以防 SSRF。
		AllowPrivate bool `yaml:"allow_private"`
		// 单次抓取超时（秒）。
		TimeoutSec int `yaml:"timeout_sec"`
	} `yaml:"fetch"`

	Backup struct {
		// 是否每日自动导出一份备份到 data/backups。
		AutoDaily bool `yaml:"auto_daily"`
		// 自动备份保留份数。
		Keep int `yaml:"keep"`
	} `yaml:"backup"`

	// 配置文件自身的路径，运行时填充。
	path string `yaml:"-"`
}

func defaults() *Config {
	c := &Config{}
	c.Server.Listen = "127.0.0.1:8080"
	c.Server.Secure = false
	c.Data.Dir = "./data"
	c.Auth.RememberDays = 30
	c.Auth.InitialAdmin.Username = "admin"
	c.Auth.AllowRegister = false
	c.Fetch.AllowPrivate = false
	c.Fetch.TimeoutSec = 8
	c.Backup.AutoDaily = true
	c.Backup.Keep = 7
	return c
}

// Load 读取配置文件；文件不存在时按默认值生成一份（含随机密钥和随机的
// 初始管理员密码）并写回磁盘。
func Load(path string) (*Config, error) {
	cfg := defaults()
	cfg.path = path

	existed := false
	raw, readErr := os.ReadFile(path)
	switch {
	case readErr == nil:
		existed = true
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("解析配置文件 %s: %w", path, err)
		}
		cfg.path = path
	case os.IsNotExist(readErr):
		cfg.Auth.InitialAdmin.Password = randomHex(9)
	default:
		return nil, fmt.Errorf("读取配置文件 %s: %w", path, readErr)
	}

	if cfg.Auth.Secret == "" {
		cfg.Auth.Secret = randomHex(32)
	}
	cfg.applyEnv()
	cfg.normalize()

	// 只在内容确实变了时才写回（首次生成、补默认值、生成密钥、环境变量覆盖）。
	// 无脑每次启动都重写，会把运维在 yaml 里加的注释一并抹掉。
	if !existed || cfg.differsFrom(raw) {
		if err := cfg.Save(); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// differsFrom 判断当前配置和磁盘原文所表达的配置是否不同。
// 比较的是「解析后再序列化」的结果，所以注释和字段顺序的差异不算数。
func (c *Config) differsFrom(raw []byte) bool {
	var onDisk Config
	if err := yaml.Unmarshal(raw, &onDisk); err != nil {
		return true
	}
	a, err1 := yaml.Marshal(&onDisk)
	b, err2 := yaml.Marshal(c)
	if err1 != nil || err2 != nil {
		return true
	}
	return !bytes.Equal(a, b)
}

// applyEnv 用 TP_ 前缀的环境变量覆盖配置，便于容器化部署。
func (c *Config) applyEnv() {
	if v := os.Getenv("TP_LISTEN"); v != "" {
		c.Server.Listen = v
	}
	if v := os.Getenv("TP_SECURE"); v != "" {
		c.Server.Secure, _ = strconv.ParseBool(v)
	}
	if v := os.Getenv("TP_DATA_DIR"); v != "" {
		c.Data.Dir = v
	}
	// 容器里没法方便地改 yaml，而这一项配错（留空）会让登录限流被伪造的
	// X-Forwarded-For 绕过，所以必须能用环境变量设。逗号分隔。
	if v := os.Getenv("TP_TRUSTED_PROXIES"); v != "" {
		c.Server.TrustedProxies = strings.Split(v, ",")
	}
	if v := os.Getenv("TP_SECRET"); v != "" {
		c.Auth.Secret = v
	}
	if v := os.Getenv("TP_ADMIN_USER"); v != "" {
		c.Auth.InitialAdmin.Username = v
	}
	if v := os.Getenv("TP_ADMIN_PASSWORD"); v != "" {
		c.Auth.InitialAdmin.Password = v
	}
	if v := os.Getenv("TP_ALLOW_PRIVATE_FETCH"); v != "" {
		c.Fetch.AllowPrivate, _ = strconv.ParseBool(v)
	}
}

func (c *Config) normalize() {
	if c.Server.Listen == "" {
		c.Server.Listen = "127.0.0.1:8080"
	}
	if c.Data.Dir == "" {
		c.Data.Dir = "./data"
	}
	if c.Auth.RememberDays <= 0 {
		c.Auth.RememberDays = 30
	}
	if c.Auth.InitialAdmin.Username == "" {
		c.Auth.InitialAdmin.Username = "admin"
	}
	if c.Fetch.TimeoutSec <= 0 {
		c.Fetch.TimeoutSec = 8
	}
	if c.Backup.Keep <= 0 {
		c.Backup.Keep = 7
	}
	c.Data.Dir = filepath.Clean(c.Data.Dir)
}

// Save 把当前配置写回磁盘（用于首次生成密钥和补齐缺省字段）。
func (c *Config) Save() error {
	if c.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	out, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	header := "# Timmypanel 配置文件。修改后需重启生效。\n"
	return os.WriteFile(c.path, []byte(header+string(out)), 0o600)
}

// DBPath 返回 sqlite 数据库文件路径。
func (c *Config) DBPath() string { return filepath.Join(c.Data.Dir, "timmypanel.db") }

// UploadDir 返回上传文件根目录。
func (c *Config) UploadDir() string { return filepath.Join(c.Data.Dir, "uploads") }

// BackupDir 返回自动备份目录。
func (c *Config) BackupDir() string { return filepath.Join(c.Data.Dir, "backups") }

// EnsureDirs 建好运行时需要的目录。
func (c *Config) EnsureDirs() error {
	for _, d := range []string{c.Data.Dir, c.UploadDir(), c.BackupDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// TrimmedProxies 去掉空白项，供 gin.SetTrustedProxies 使用。
func (c *Config) TrimmedProxies() []string {
	out := make([]string, 0, len(c.Server.TrustedProxies))
	for _, p := range c.Server.TrustedProxies {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
