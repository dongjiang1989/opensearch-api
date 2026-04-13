package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/api/router"
	"github.com/dongjiang1989/opensearch-api/internal/config"
	"github.com/dongjiang1989/opensearch-api/internal/embedding"
	"github.com/dongjiang1989/opensearch-api/internal/indexer"
	"github.com/dongjiang1989/opensearch-api/internal/opensearch"
	"github.com/dongjiang1989/opensearch-api/internal/storage"
	"github.com/dongjiang1989/opensearch-api/internal/tenant"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	configPath string
)

func init() {
	flag.StringVar(&configPath, "config", "", "path to config.yaml file")
}

func main() {
	// 解析命令行参数
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	logger, err := initLogger(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			log.Printf("Failed to sync logger: %v", err)
		}
	}()

	logger.Info("starting opensearch-file-api",
		zap.String("version", Version),
		zap.String("build_time", BuildTime),
		zap.String("mode", cfg.Server.Mode))

	// 初始化 OpenSearch 客户端
	osClient, err := opensearch.NewClient(&cfg.OpenSearch, logger)
	if err != nil {
		logger.Fatal("failed to create opensearch client", zap.Error(err))
	}

	// 检查 OpenSearch 连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := osClient.Ping(ctx); err != nil {
		logger.Fatal("opensearch ping failed", zap.Error(err))
	}
	logger.Info("connected to opensearch", zap.String("url", cfg.OpenSearch.URL()))

	// 初始化存储
	var fileStorage storage.Storage
	if cfg.Storage.IsS3() {
		fileStorage, err = storage.NewS3Storage(storage.S3StorageConfig{
			Bucket:   cfg.Storage.S3Bucket,
			Region:   cfg.Storage.S3Region,
			Endpoint: cfg.Storage.S3Endpoint,
			KeyID:    cfg.Storage.S3KeyID,
			Secret:   cfg.Storage.S3Secret,
			Logger:   logger,
		})
		if err != nil {
			logger.Fatal("failed to create s3 storage", zap.Error(err))
		}
		logger.Info("using S3 storage", zap.String("bucket", cfg.Storage.S3Bucket))
	} else {
		fileStorage, err = storage.NewLocalStorage(storage.LocalStorageConfig{
			BasePath: cfg.Storage.LocalPath,
			Logger:   logger,
		})
		if err != nil {
			logger.Fatal("failed to create local storage", zap.Error(err))
		}
		logger.Info("using local storage", zap.String("path", cfg.Storage.LocalPath))
	}

	// 初始化内容提取器
	extractor := storage.NewCompositeExtractor(
		storage.NewPDFExtractor(),
		storage.NewImageExtractor(storage.ImageExtractorConfig{
			EnableOCR: cfg.Storage.ImageOCR,
			OCRLang:   cfg.Storage.ImageOCRLang,
		}),
		storage.NewTextExtractor(storage.TextExtractorConfig{MaxSize: 10 * 1024 * 1024}),
		storage.NewDocumentExtractor(),
	)

	// 初始化嵌入服务
	var textEmbedder embedding.EmbeddingModel
	var clipModel embedding.EmbeddingModel

	switch cfg.Embedding.Provider {
	case "openai":
		textEmbedder = embedding.NewOpenAIEmbedding(embedding.OpenAIEmbeddingConfig{
			APIKey:     cfg.Embedding.APIKey,
			APIURL:     cfg.Embedding.APIURL,
			Model:      cfg.Embedding.Model,
			Dimensions: cfg.Embedding.Dimensions,
			Timeout:    time.Duration(cfg.Embedding.Timeout) * time.Second,
		})
		logger.Info("using OpenAI embedding service",
			zap.String("model", cfg.Embedding.Model),
			zap.Int("dimensions", cfg.Embedding.Dimensions))
	case "local":
		textEmbedder = embedding.NewLocalEmbedding(embedding.LocalEmbeddingConfig{
			APIURL:     cfg.Embedding.APIURL,
			Model:      cfg.Embedding.Model,
			Dimensions: cfg.Embedding.Dimensions,
			Timeout:    time.Duration(cfg.Embedding.Timeout) * time.Second,
		})
		logger.Info("using local embedding service",
			zap.String("model", cfg.Embedding.Model),
			zap.String("api_url", cfg.Embedding.APIURL))
	case "clip":
		clipModel = embedding.NewCLIPEmbedding(embedding.CLIPEmbeddingConfig{
			APIURL:     cfg.Embedding.APIURL,
			Dimensions: cfg.Embedding.Dimensions,
			Timeout:    time.Duration(cfg.Embedding.Timeout) * time.Second,
		})
		logger.Info("using CLIP multimodal embedding service",
			zap.Int("dimensions", cfg.Embedding.Dimensions))
	default:
		logger.Info("no embedding service configured, vector search disabled")
	}

	// 初始化索引器
	fileIndexer := indexer.NewIndexer(indexer.IndexerConfig{
		OpenSearch: osClient,
		Storage:    fileStorage,
		Extractor:  extractor,
		Embedder:   textEmbedder,
		ClipModel:  clipModel,
		Logger:     logger,
	})

	// 初始化租户服务
	tenantRepo := tenant.NewInMemoryRepository()
	tenantService := tenant.NewService(tenant.ServiceConfig{
		Repository:    tenantRepo,
		OpenSearch:    osClient,
		Logger:        logger,
		IndexMappings: opensearch.FileMapping(),
	})

	// 设置路由
	r := router.Setup(router.Config{
		OpenSearch:    osClient,
		TenantService: tenantService,
		Indexer:       fileIndexer,
		Logger:        logger,
		Mode:          cfg.Server.Mode,
	})

	// 启动服务器
	srv := &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// 优雅关闭
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		logger.Info("shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Fatal("server shutdown failed", zap.Error(err))
		}
	}()

	logger.Info("server started", zap.String("address", cfg.Server.Address()))

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("server failed", zap.Error(err))
	}

	logger.Info("server stopped")
}

func initLogger(cfg *config.Config) (*zap.Logger, error) {
	var zcfg zap.Config

	switch cfg.Log.Format {
	case "json":
		zcfg = zap.NewProductionConfig()
	case "console":
		zcfg = zap.NewDevelopmentConfig()
	default:
		zcfg = zap.NewProductionConfig()
	}

	switch cfg.Log.Level {
	case "debug":
		zcfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zcfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zcfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zcfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zcfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	return zcfg.Build()
}
