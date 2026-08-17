// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

// Timmypanel 是一个自用的网址导航站：登录后把常用网站以卡片形式聚合到首页。
// 前端资源编译进二进制，运行时只依赖一个 sqlite 文件。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"timmypanel/internal/api"
	"timmypanel/internal/config"
	"timmypanel/internal/model"
	"timmypanel/internal/web"
)

// Version 由构建脚本通过 -ldflags 注入。
var Version = "dev"

func main() {
	configPath := flag.String("config", "./data/config.yaml", "配置文件路径")
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	debug := flag.Bool("debug", false, "开启调试日志和 gin 调试模式")
	resetUser := flag.String("reset-password", "", "重置指定用户的密码，打印新密码后退出")
	flag.Parse()

	if *showVersion {
		fmt.Println("timmypanel", Version)
		return
	}

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	if *resetUser != "" {
		if err := resetPassword(*configPath, *resetUser); err != nil {
			slog.Error("重置密码失败", "err", err)
			os.Exit(1)
		}
		return
	}

	if err := run(*configPath, *debug); err != nil {
		slog.Error("启动失败", "err", err)
		os.Exit(1)
	}
}

func resetPassword(configPath, username string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	db, err := model.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()
	plain, err := model.ResetPassword(db, username)
	if err != nil {
		return err
	}
	fmt.Printf("已重置用户 %s 的密码\n新密码: %s\n请登录后立即修改。该密码不会写入配置文件。\n", username, plain)
	return nil
}

func run(configPath string, debug bool) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if err := cfg.EnsureDirs(); err != nil {
		return err
	}

	db, err := model.Open(cfg.DBPath())
	if err != nil {
		return err
	}
	created, err := model.EnsureInitialAdmin(db, cfg.Auth.InitialAdmin.Username, cfg.Auth.InitialAdmin.Password)
	if err != nil {
		return err
	}
	if created {
		pwd := cfg.Auth.InitialAdmin.Password
		slog.Info(strings.Repeat("=", 56))
		slog.Info("已创建初始管理员账号，请登录后立即修改密码",
			"用户名", cfg.Auth.InitialAdmin.Username, "密码", pwd)
		slog.Info("该密码也保存在 " + configPath + "，改完密码后可以把它清空")
		slog.Info(strings.Repeat("=", 56))
	}
	model.StartSessionGC(db)

	if !debug {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	if debug {
		r.Use(gin.Logger())
	}
	// 默认不信任任何代理头；配置了 trusted_proxies 才按代理链取真实 IP，
	// 否则限流会因为伪造的 X-Forwarded-For 形同虚设。
	if err := r.SetTrustedProxies(cfg.TrimmedProxies()); err != nil {
		return err
	}
	r.MaxMultipartMemory = 16 << 20

	server := api.NewServer(db, cfg)
	server.Register(r)
	server.StartAutoBackup()
	if err := web.Register(r); err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		// WriteTimeout 要比一次 /sites/parse 或 /sites/backfill 的最坏耗时长：
		// 20 条 × 两次出站 × fetch.timeout_sec，5 路并发，大约一分多钟。
		WriteTimeout: 2 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Timmypanel 已启动", "版本", Version, "监听", cfg.Server.Listen, "数据目录", cfg.Data.Dir)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP 服务异常退出", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	slog.Info("正在关闭…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		return err
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	slog.Info("已退出")
	return nil
}
