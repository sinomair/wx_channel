package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/qtgolang/SunnyNet/SunnyNet"
	"github.com/qtgolang/SunnyNet/public"

	"wx_channel/internal/api"
	"wx_channel/internal/assets"
	"wx_channel/internal/config"
	"wx_channel/internal/database"
	"wx_channel/internal/handlers"
	"wx_channel/internal/storage"
	"wx_channel/internal/utils"
	"wx_channel/internal/websocket"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/proxy"
)

var Sunny = SunnyNet.NewSunny()
var cfg *config.Config
var v string = "?t=5.3.0" // Default version query
var port int
var currentPageURL = "" // 存储当前页面的完整URL
var logInitMsg string

// 全局管理器
var (
	csvManager        *storage.CSVManager
	fileManager       *storage.FileManager
	apiHandler        *handlers.APIHandler
	uploadHandler     *handlers.UploadHandler
	recordHandler     *handlers.RecordHandler
	scriptHandler     *handlers.ScriptHandler
	batchHandler      *handlers.BatchHandler
	commentHandler    *handlers.CommentHandler
	consoleAPIHandler *handlers.ConsoleAPIHandler
	webSocketHandler  *handlers.WebSocketHandler

	// WebSocket 和 API 服务
	wsHub         *websocket.Hub
	searchService *api.SearchService
)

// downloadRecordsHeader CSV 文件的表头
var downloadRecordsHeader = []string{"ID", "标题", "视频号名称", "视频号分类", "公众号名称", "视频链接", "页面链接", "文件大小", "时长", "阅读量", "点赞量", "评论量", "收藏数", "转发数", "创建时间", "IP所在地", "下载时间", "页面来源", "搜索关键词"}

// initDownloadRecords 初始化下载记录系统
func initDownloadRecords() error {
	downloadsDir, err := utils.ResolveDownloadDir(cfg.DownloadsDir)
	if err != nil {
		return fmt.Errorf("解析下载目录失败: %v", err)
	}

	fileManager, err = storage.NewFileManager(downloadsDir)
	if err != nil {
		return fmt.Errorf("创建文件管理器失败: %v", err)
	}

	csvPath := filepath.Join(downloadsDir, cfg.RecordsFile)
	csvManager, err = storage.NewCSVManager(csvPath, downloadRecordsHeader)
	if err != nil {
		return fmt.Errorf("创建CSV管理器失败: %v", err)
	}

	return nil
}

// saveDynamicHTML 保存动态页面的完整HTML内容
func saveDynamicHTML(htmlContent string, parsedURL *url.URL, fullURL string, timestamp int64) {
	if fileManager == nil || cfg == nil {
		utils.Warn("文件管理器或配置未初始化，无法保存页面内容: %s", fullURL)
		return
	}
	if !cfg.SavePageSnapshot {
		return
	}
	if htmlContent == "" || parsedURL == nil {
		return
	}

	if cfg.SaveDelay > 0 {
		time.Sleep(cfg.SaveDelay)
	}

	saveTime := time.Now()
	if timestamp > 0 {
		saveTime = time.Unix(0, timestamp*int64(time.Millisecond))
	}

	downloadsDir, err := utils.ResolveDownloadDir(cfg.DownloadsDir)
	if err != nil {
		utils.HandleError(err, "解析下载目录用于保存页面内容")
		return
	}

	if err := utils.EnsureDir(downloadsDir); err != nil {
		utils.HandleError(err, "创建下载目录用于保存页面内容")
		return
	}

	pagesRoot := filepath.Join(downloadsDir, "page_snapshots")
	if err := utils.EnsureDir(pagesRoot); err != nil {
		utils.HandleError(err, "创建页面保存根目录")
		return
	}

	dateDir := filepath.Join(pagesRoot, saveTime.Format("2006-01-02"))
	if err := utils.EnsureDir(dateDir); err != nil {
		utils.HandleError(err, "创建页面保存日期目录")
		return
	}

	var filenameParts []string
	if parsedURL.Path != "" && parsedURL.Path != "/" {
		segments := strings.Split(parsedURL.Path, "/")
		for _, segment := range segments {
			segment = strings.TrimSpace(segment)
			if segment == "" || segment == "." {
				continue
			}
			filenameParts = append(filenameParts, utils.CleanFilename(segment))
		}
	}

	if parsedURL.RawQuery != "" {
		querySegment := strings.ReplaceAll(parsedURL.RawQuery, "&", "_")
		querySegment = strings.ReplaceAll(querySegment, "=", "-")
		querySegment = utils.CleanFilename(querySegment)
		if querySegment != "" {
			filenameParts = append(filenameParts, querySegment)
		}
	}

	if len(filenameParts) == 0 {
		filenameParts = append(filenameParts, "page")
	}

	baseName := strings.Join(filenameParts, "_")
	fileName := fmt.Sprintf("%s_%s.html", saveTime.Format("150405"), baseName)
	targetPath := utils.GenerateUniqueFilename(dateDir, fileName, 100)

	if err := os.WriteFile(targetPath, []byte(htmlContent), 0644); err != nil {
		utils.HandleError(err, "保存页面HTML内容")
		return
	}

	metaData := map[string]interface{}{
		"url":       fullURL,
		"host":      parsedURL.Host,
		"path":      parsedURL.Path,
		"query":     parsedURL.RawQuery,
		"saved_at":  saveTime.Format(time.RFC3339),
		"timestamp": timestamp,
	}

	metaBytes, err := json.MarshalIndent(metaData, "", "  ")
	if err == nil {
		metaPath := strings.TrimSuffix(targetPath, filepath.Ext(targetPath)) + ".meta.json"
		if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
			utils.HandleError(err, "保存页面元数据")
		}
	}

	utils.LogInfo("[页面快照] 已保存: %s", targetPath)

	utils.PrintSeparator()
	color.Blue("💾 页面快照已保存")
	utils.PrintSeparator()
	utils.PrintLabelValue("📁", "保存路径", targetPath)
	utils.PrintLabelValue("🔗", "页面链接", fullURL)
	utils.PrintSeparator()
	fmt.Println()
	fmt.Println()
}

func printDownloadRecordInfo() {
	utils.PrintSeparator()
	color.Blue("📋 下载记录信息")
	utils.PrintSeparator()

	downloadsDir, err := utils.ResolveDownloadDir(cfg.DownloadsDir)
	if err != nil {
		utils.HandleError(err, "解析下载目录")
		return
	}

	recordsPath := filepath.Join(downloadsDir, cfg.RecordsFile)
	utils.PrintLabelValue("📁", "记录文件", recordsPath)
	utils.PrintLabelValue("✏️", "记录格式", "CSV表格格式")
	utils.PrintLabelValue("📊", "记录字段", strings.Join(downloadRecordsHeader, ", "))
	utils.PrintSeparator()
}

func printEnvConfig() {
	hasAnyConfig := os.Getenv("WX_CHANNEL_TOKEN") != "" ||
		os.Getenv("WX_CHANNEL_ALLOWED_ORIGINS") != "" ||
		os.Getenv("WX_CHANNEL_LOG_FILE") != "" ||
		os.Getenv("WX_CHANNEL_LOG_MAX_MB") != "" ||
		os.Getenv("WX_CHANNEL_SAVE_PAGE_SNAPSHOT") != "" ||
		os.Getenv("WX_CHANNEL_SAVE_SEARCH_DATA") != "" ||
		os.Getenv("WX_CHANNEL_SAVE_PAGE_JS") != "" ||
		os.Getenv("WX_CHANNEL_SHOW_LOG_BUTTON") != "" ||
		os.Getenv("WX_CHANNEL_UPLOAD_CHUNK_CONCURRENCY") != "" ||
		os.Getenv("WX_CHANNEL_UPLOAD_MERGE_CONCURRENCY") != "" ||
		os.Getenv("WX_CHANNEL_DOWNLOAD_CONCURRENCY") != ""

	if hasAnyConfig {
		utils.PrintSeparator()
		color.Blue("⚙️  环境变量配置信息")
		utils.PrintSeparator()

		if cfg.SecretToken != "" {
			utils.PrintLabelValue("🔐", "安全令牌", "已设置")
		}
		if len(cfg.AllowedOrigins) > 0 {
			utils.PrintLabelValue("🌐", "允许的Origin", strings.Join(cfg.AllowedOrigins, ", "))
		}
		if cfg.LogFile != "" {
			utils.PrintLabelValue("📝", "日志文件", cfg.LogFile)
		}
		if cfg.MaxLogSizeMB > 0 {
			utils.PrintLabelValue("📊", "日志最大大小", fmt.Sprintf("%d MB", cfg.MaxLogSizeMB))
		}
		utils.PrintLabelValue("💾", "保存页面快照", fmt.Sprintf("%v", cfg.SavePageSnapshot))
		utils.PrintLabelValue("🔍", "保存搜索数据", fmt.Sprintf("%v", cfg.SaveSearchData))
		utils.PrintLabelValue("📄", "保存JS文件", fmt.Sprintf("%v", cfg.SavePageJS))
		utils.PrintLabelValue("🖼️", "显示日志按钮", fmt.Sprintf("%v", cfg.ShowLogButton))
		utils.PrintLabelValue("📤", "分片上传并发", cfg.UploadChunkConcurrency)
		utils.PrintLabelValue("🔀", "分片合并并发", cfg.UploadMergeConcurrency)
		utils.PrintLabelValue("📥", "批量下载并发", cfg.DownloadConcurrency)
		utils.PrintSeparator()
	}
}

func printTitle() {
	color.Set(color.FgCyan)
	fmt.Println("")
	fmt.Println(" ██╗    ██╗██╗  ██╗     ██████╗██╗  ██╗ █████╗ ███╗   ██╗███╗   ██╗███████╗██╗     ")
	fmt.Println(" ██║    ██║╚██╗██╔╝    ██╔════╝██║  ██║██╔══██╗████╗  ██║████╗  ██║██╔════╝██║     ")
	fmt.Println(" ██║ █╗ ██║ ╚███╔╝     ██║     ███████║███████║██╔██╗ ██║██╔██╗ ██║█████╗  ██║     ")
	fmt.Println(" ██║███╗██║ ██╔██╗     ██║     ██╔══██║██╔══██║██║╚██╗██║██║╚██╗██║██╔══╝  ██║     ")
	fmt.Println(" ╚███╔███╔╝██╔╝ ██╗    ╚██████╗██║  ██║██║  ██║██║ ╚████║██║ ╚████║███████╗███████╗")
	fmt.Println("  ╚══╝╚══╝ ╚═╝  ╚═╝     ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═══╝╚══════╝╚══════╝")
	color.Unset()

	color.Yellow("    微信视频号下载助手 v%s", cfg.Version)
	color.Yellow("    项目地址：https://github.com/nobiyou/wx_channel")
	color.Green("    v%s 更新要点：", cfg.Version)
	color.Green("    • 通用批量下载组件 - 统一UI，减少400+行代码")
	color.Green("    • Home页面分类视频批量下载 - 支持美食、生活等分类")
	color.Green("    • 视频列表优化 - 完整信息显示，分页浏览")
	color.Green("    • 下载功能增强 - 强制重下、取消、实时进度")
	color.Green("    • 搜索页面增强 - 显示直播数据，HTML标签清理")
	color.Green("    • Bug修复 - 下载显示、复选框、标题清理等")
	fmt.Println()
}

// Run 启动应用
func Run(cfgParam *config.Config) {
	cfg = cfgParam // 设置全局配置
	// 记录配置加载
	utils.LogConfigLoad("config.yaml", true)

	// 初始化日志（可选滚动）
	if cfg.LogFile != "" {
		_ = utils.InitLoggerWithRotation(utils.INFO, cfg.LogFile, cfg.MaxLogSizeMB)
		logInitMsg = fmt.Sprintf("日志已初始化: %s (最大 %dMB)", cfg.LogFile, cfg.MaxLogSizeMB)
	}
	port = cfg.Port
	v = "?t=" + cfg.Version

	os_env := runtime.GOOS

	// 原有的参数解析逻辑已移除，由 Cobra 处理并传入 cfg
	// 确保端口设置正确
	Sunny.SetPort(port)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signalChan
		color.Red("\n正在关闭服务...%v\n\n", sig)
		utils.LogSystemShutdown(fmt.Sprintf("收到信号: %v", sig))
		database.Close()
		if os_env == "darwin" {
			proxy.DisableProxyInMacOS(proxy.ProxySettings{
				Device:   "", // dev parameter needs to be passed down if needed
				Hostname: "127.0.0.1",
				Port:     strconv.Itoa(port),
			})
		}
		os.Exit(0)
	}()

	printTitle()

	if err := initDownloadRecords(); err != nil {
		utils.HandleError(err, "初始化下载记录系统")
	} else {
		printDownloadRecordInfo()
		if logInitMsg != "" {
			utils.Info(logInitMsg)
			logInitMsg = ""
		}
	}

	printEnvConfig()

	apiHandler = handlers.NewAPIHandler(cfg)

	if csvManager != nil {
		uploadHandler = handlers.NewUploadHandler(cfg, csvManager)
		recordHandler = handlers.NewRecordHandler(cfg, csvManager)
	}

	// 使用 assets 包中的资源
	scriptHandler = handlers.NewScriptHandler(cfg, assets.CoreJS, assets.DecryptJS, assets.DownloadJS, assets.HomeJS, assets.FeedJS, assets.ProfileJS, assets.SearchJS, assets.BatchDownloadJS, assets.ZipJS, assets.FileSaverJS, assets.MittJS, assets.EventbusJS, assets.UtilsJS, assets.APIClientJS, v)

	if csvManager != nil {
		batchHandler = handlers.NewBatchHandler(cfg, csvManager)
	}

	commentHandler = handlers.NewCommentHandler(cfg)

	downloadsDir, err := utils.ResolveDownloadDir(cfg.DownloadsDir)
	if err != nil {
		utils.HandleError(err, "解析下载目录用于数据库初始化")
	} else {
		dbPath := filepath.Join(downloadsDir, "console.db")
		if err := database.Initialize(&database.Config{DBPath: dbPath}); err != nil {
			utils.HandleError(err, "初始化数据库")
			utils.Warn("Web控制台功能可能受限")
		} else {
			utils.Info("✓ 数据库已初始化: %s", dbPath)
			settingsRepo := database.NewSettingsRepository()
			// 注意：此时 config 包的 dbLoader 是全局的，这里设了，但是 cfg 实例可能是旧的
			// 但我们通过 reload 刷新
			// config.SetDatabaseLoader(settingsRepo) -> config is imported
			config.SetDatabaseLoader(settingsRepo)

			// 重新加载配置
			cfg = config.Reload()
			utils.Info("✓ 配置已从数据库重新加载")

			if err := initDownloadRecords(); err != nil {
				utils.HandleError(err, "重新初始化下载记录系统")
			} else {
				utils.Info("✓ 下载记录系统已使用新配置重新初始化")
				if csvManager != nil {
					uploadHandler = handlers.NewUploadHandler(cfg, csvManager)
					recordHandler = handlers.NewRecordHandler(cfg, csvManager)
					batchHandler = handlers.NewBatchHandler(cfg, csvManager)
					utils.Info("✓ 处理器已使用新配置重新初始化")
				}
			}
		}
	}

	consoleAPIHandler = handlers.NewConsoleAPIHandler(cfg)
	webSocketHandler = handlers.NewWebSocketHandler()

	existing, err1 := certificate.CheckCertificate("SunnyNet")
	if err1 != nil {
		utils.HandleError(err1, "检查证书")
		utils.Warn("程序将继续运行，但HTTPS功能可能受限...")
		existing = false
	} else if !existing {
		utils.Info("正在安装证书...")
		err := certificate.InstallCertificate(assets.CertData)
		time.Sleep(cfg.CertInstallDelay)
		if err != nil {
			utils.HandleError(err, "证书安装")
			utils.Warn("如需完整功能，请手动安装证书或以管理员身份运行程序。")

			if fileManager != nil {
				downloadsDir, err := utils.ResolveDownloadDir(cfg.DownloadsDir)
				if err == nil {
					certPath := filepath.Join(downloadsDir, cfg.CertFile)
					if err := utils.EnsureDir(downloadsDir); err == nil {
						if err := os.WriteFile(certPath, assets.CertData, 0644); err == nil {
							utils.Info("证书文件已保存到: %s", certPath)
						}
					}
				}
			}
		} else {
			utils.Info("✓ 证书安装成功！")
		}
	} else {
		utils.Info("✓ 证书已存在，无需重新安装。")
	}

	Sunny.SetGoCallback(HttpCallback, nil, nil, nil)
	sunnyErr := Sunny.Start().Error
	if sunnyErr != nil {
		utils.HandleError(sunnyErr, "启动代理服务")
		utils.Warn("按 Ctrl+C 退出...")
		select {}
	}

	proxy_server := fmt.Sprintf("127.0.0.1:%v", port)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(&url.URL{
				Scheme: "http",
				Host:   proxy_server,
			}),
		},
	}
	_, err3 := client.Get("https://sunny.io/")
	if err3 == nil {
		if os_env == "windows" {
			ok := Sunny.StartProcess()
			if !ok {
				color.Red("\nERROR 启动进程代理失败，检查是否以管理员身份运行\n")
				color.Yellow("按 Ctrl+C 退出...\n")
				select {}
			}
			Sunny.ProcessAddName("WeChatAppEx.exe")
		}

		utils.PrintSeparator()
		color.Blue("📡 服务状态信息")
		utils.PrintSeparator()
		utils.PrintLabelValue("⏳", "服务状态", "已启动")
		utils.PrintLabelValue("🔌", "代理端口", port)
		utils.PrintLabelValue("📱", "支持平台", "微信视频号")

		proxyMode := "进程代理"
		if os_env != "windows" {
			proxyMode = "系统代理"
		}
		utils.LogSystemStart(port, proxyMode)

		wsHub = websocket.NewHub()
		go wsHub.Run()
		searchService = api.NewSearchService(wsHub)
		utils.Info("✓ WebSocket Hub 已初始化")

		wsPort := port + 1
		go startWebSocketServer(wsPort)

		utils.Info("🔍 请打开需要下载的视频号页面进行下载")
	} else {
		utils.PrintSeparator()
		utils.Warn("⚠️ 您还未安装证书，请在浏览器打开 http://%v 并根据说明安装证书", proxy_server)
		utils.Warn("⚠️ 在安装完成后重新启动此程序即可")
		utils.PrintSeparator()
	}
	utils.Info("💡 服务正在运行，按 Ctrl+C 退出...")
	select {}
}

// HttpCallback 处理 HTTP 回调
func HttpCallback(Conn *SunnyNet.HttpConn) {
	host := Conn.Request.URL.Hostname()
	path := Conn.Request.URL.Path
	if Conn.Type == public.HttpSendRequest {
		Conn.Request.Header.Del("Accept-Encoding")

		// 使用 assets 中的资源
		if handlers.HandleStaticFiles(Conn, assets.ZipJS, assets.FileSaverJS) {
			return
		}

		if apiHandler != nil {
			if apiHandler.HandleProfile(Conn) {
				return
			}
			if apiHandler.HandleTip(Conn) {
				return
			}
			if apiHandler.HandlePageURL(Conn) {
				currentPageURL = apiHandler.GetCurrentURL()
				if recordHandler != nil {
					recordHandler.SetCurrentURL(currentPageURL)
				}
				return
			}
		}

		if uploadHandler != nil {
			if uploadHandler.HandleInitUpload(Conn) {
				return
			}
			if uploadHandler.HandleUploadChunk(Conn) {
				return
			}
			if uploadHandler.HandleCompleteUpload(Conn) {
				return
			}
			if uploadHandler.HandleUploadStatus(Conn) {
				return
			}
			if uploadHandler.HandleSaveVideo(Conn) {
				return
			}
			if uploadHandler.HandleSaveCover(Conn) {
				return
			}
			if uploadHandler.HandleDownloadVideo(Conn) {
				return
			}
		}

		if recordHandler != nil {
			if recordHandler.HandleRecordDownload(Conn) {
				return
			}
			if recordHandler.HandleExportVideoList(Conn) {
				return
			}
			if recordHandler.HandleExportVideoListJSON(Conn) {
				return
			}
			if recordHandler.HandleExportVideoListMarkdown(Conn) {
				return
			}
			if recordHandler.HandleBatchDownloadStatus(Conn) {
				return
			}
		}

		if batchHandler != nil {
			if batchHandler.HandleBatchStart(Conn) {
				return
			}
			if batchHandler.HandleBatchProgress(Conn) {
				return
			}
			if batchHandler.HandleBatchCancel(Conn) {
				return
			}
			if batchHandler.HandleBatchResume(Conn) {
				return
			}
			if batchHandler.HandleBatchClear(Conn) {
				return
			}
			if batchHandler.HandleBatchFailed(Conn) {
				return
			}
		}

		if commentHandler != nil {
			if commentHandler.HandleSaveCommentData(Conn) {
				return
			}
		}

		if path == "/console" || path == "/console/" {
			// Console html not embedded? main.go read it from file "web/console.html"
			// If we want to keep it simple we assume web folder is there
			consoleHTML, err := os.ReadFile("web/console.html")
			if err != nil {
				utils.Warn("无法读取 web/console.html: %v", err)
				Conn.StopRequest(404, "Console not found", http.Header{})
				return
			}
			headers := http.Header{}
			headers.Set("Content-Type", "text/html; charset=utf-8")
			Conn.StopRequest(200, string(consoleHTML), headers)
			return
		}

		isWeixinResource := strings.Contains(path, "pic_blank.gif") ||
			strings.Contains(path, "we-emoji") ||
			strings.Contains(path, "Expression") ||
			strings.Contains(path, "auth_icon") ||
			strings.Contains(path, "weixin/checkresupdate") ||
			strings.Contains(path, "fed_upload") ||
			strings.HasPrefix(path, "/a/") ||
			strings.HasPrefix(path, "/weixin/")

		if !isWeixinResource && (strings.HasPrefix(path, "/js/") || strings.HasPrefix(path, "/css/") || strings.HasPrefix(path, "/docs/") ||
			strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".jpg") ||
			strings.HasSuffix(path, ".jpeg") || strings.HasSuffix(path, ".gif") ||
			strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".ico") ||
			strings.HasSuffix(path, ".md")) {
			filePath := "web" + path
			content, err := os.ReadFile(filePath)
			if err != nil {
				return
			}
			headers := http.Header{}
			if strings.HasSuffix(path, ".js") {
				headers.Set("Content-Type", "application/javascript; charset=utf-8")
			} else if strings.HasSuffix(path, ".css") {
				headers.Set("Content-Type", "text/css; charset=utf-8")
			}
			Conn.StopRequest(200, string(content), headers)
			return
		}

		if strings.HasPrefix(path, "/api/") && consoleAPIHandler != nil {
			handleConsoleAPI(Conn)
			return
		}

		if strings.HasPrefix(path, "/__wx_channels_api/") && Conn.Request.Method == "OPTIONS" {
			headers := http.Header{}
			headers.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			headers.Set("Access-Control-Allow-Headers", "Content-Type, X-Local-Auth")
			if cfg != nil && len(cfg.AllowedOrigins) > 0 {
				origin := Conn.Request.Header.Get("Origin")
				for _, o := range cfg.AllowedOrigins {
					if o == origin {
						headers.Set("Access-Control-Allow-Origin", origin)
						headers.Set("Vary", "Origin")
						break
					}
				}
			}
			Conn.StopRequest(204, "", headers)
			return
		}

		if path == "/__wx_channels_api/save_page_content" {
			var contentData struct {
				URL       string `json:"url"`
				HTML      string `json:"html"`
				Timestamp int64  `json:"timestamp"`
			}
			body, err := io.ReadAll(Conn.Request.Body)
			if err != nil {
				utils.HandleError(err, "读取save_page_content请求体")
				return
			}
			if err := Conn.Request.Body.Close(); err != nil {
				utils.HandleError(err, "关闭请求体")
			}
			err = json.Unmarshal(body, &contentData)
			if err != nil {
				utils.HandleError(err, "解析页面内容数据")
			} else {
				parsedURL, err := url.Parse(contentData.URL)
				if err != nil {
					utils.HandleError(err, "解析页面内容URL")
				} else {
					saveDynamicHTML(contentData.HTML, parsedURL, contentData.URL, contentData.Timestamp)
				}
			}
			headers := http.Header{}
			headers.Set("Content-Type", "application/json")
			headers.Set("__debug", "fake_resp")
			Conn.StopRequest(200, "{}", headers)
			return
		}
	}
	if Conn.Type == public.HttpResponseOK {
		if Conn.Response.Body != nil {
			Body, _ := io.ReadAll(Conn.Response.Body)
			_ = Conn.Response.Body.Close()

			if strings.Contains(path, ".js") {
				contentType := strings.ToLower(Conn.Response.Header.Get("content-type"))
				utils.LogInfo("[响应] Path=%s | ContentType=%s", path, contentType)
			}

			if scriptHandler != nil {
				if scriptHandler.HandleHTMLResponse(Conn, host, path, Body) {
					return
				}
			}

			if scriptHandler != nil {
				if scriptHandler.HandleJavaScriptResponse(Conn, host, path, Body) {
					return
				}
			}

			Conn.Response.Body = io.NopCloser(bytes.NewBuffer(Body))
		}
	}
}

// Helpers needed implicitly
type SunnyNetResponseWriter struct {
	conn       *SunnyNet.HttpConn
	headers    http.Header
	statusCode int
	body       bytes.Buffer
}

func NewSunnyNetResponseWriter(conn *SunnyNet.HttpConn) *SunnyNetResponseWriter {
	return &SunnyNetResponseWriter{
		conn:       conn,
		headers:    make(http.Header),
		statusCode: http.StatusOK,
	}
}

func (w *SunnyNetResponseWriter) Header() http.Header {
	return w.headers
}

func (w *SunnyNetResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *SunnyNetResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *SunnyNetResponseWriter) Flush() {
	w.conn.StopRequest(w.statusCode, w.body.String(), w.headers)
}

func handleConsoleAPI(Conn *SunnyNet.HttpConn) {
	w := NewSunnyNetResponseWriter(Conn)
	consoleAPIHandler.HandleAPIRequest(w, Conn.Request)
	w.Flush()
}

func startWebSocketServer(wsPort int) {
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		handlers.ServeWs(w, r)
	})

	wsHandler := websocket.NewHandler(wsHub)
	mux.HandleFunc("/ws/api", wsHandler.ServeHTTP)

	mux.HandleFunc("/ws/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		hub := handlers.GetWebSocketHub()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"clients": hub.ClientCount(),
		})
	})

	if searchService != nil {
		mux.HandleFunc("/api/channels/contact/search", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			searchService.SearchContact(w, r)
		})

		mux.HandleFunc("/api/channels/contact/feed/list", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			searchService.GetFeedList(w, r)
		})

		mux.HandleFunc("/api/channels/feed/profile", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			searchService.GetFeedProfile(w, r)
		})

		mux.HandleFunc("/api/channels/status", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			searchService.GetStatus(w, r)
		})
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", wsPort),
		Handler: mux,
	}

	utils.Info("🔌 WebSocket服务已启动，端口: %d", wsPort)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		utils.Warn("WebSocket服务启动失败: %v", err)
	}
}
