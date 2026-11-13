// 煤矿安全员AI智能体平台：单体服务入口。
//
// 一个进程同时承担 HTTP 接口与后台任务轮询，编译为单文件二进制，
// 静态资源后续由 go:embed 打入同一二进制。
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/api"
	"coalrag/internal/auth"
	"coalrag/internal/bootstrap"
	"coalrag/internal/config"
	"coalrag/internal/db"
	"coalrag/internal/llm"
	"coalrag/internal/logx"
	"coalrag/internal/search"
	"coalrag/internal/service"
	"coalrag/internal/store"
	"coalrag/internal/vector"
	"coalrag/internal/wx"
	"coalrag/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := logx.New(cfg.LogLevel)
	if cfg.LogLevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Open(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()

	applied, err := db.Migrate(ctx, pool)
	if err != nil {
		return err
	}
	if len(applied) > 0 {
		logger.Info("数据库迁移完成", "versions", applied)
	}

	st := store.New(pool)
	if cfg.Bootstrap {
		done, err := bootstrap.Run(ctx, st)
		if err != nil {
			return err
		}
		if done {
			logger.Info("空库初始化完成", "admin", "admin / Admin@123456")
		}
	}

	chat, embedder, err := buildModels(cfg)
	if err != nil {
		return err
	}
	if err := bootstrap.CheckEmbedDim(ctx, st, cfg.Embed.Dim, embedder.Name()); err != nil {
		return err
	}

	vec, err := buildVectorStore(cfg, st)
	if err != nil {
		return err
	}

	svc := &service.Service{
		Store:    st,
		Cfg:      cfg,
		Chat:     chat,
		Embedder: embedder,
		Vector:   vec,
		Lexical:  search.NewIndex(),
		Log:      logger,
	}
	if err := svc.EnsureDirs(); err != nil {
		return err
	}
	if err := svc.ReloadIndex(ctx); err != nil {
		return fmt.Errorf("装载检索索引: %w", err)
	}
	logger.Info("检索索引就绪", "impl", vec.Kind(), "chunks", vec.Size(), "embed", embedder.Name(), "dim", cfg.Embed.Dim)

	policies, err := st.ListPolicies(ctx)
	if err != nil {
		return err
	}

	mgr := auth.NewManager(cfg.JWTSecret, cfg.JWTTTL.D())
	srv := api.New(cfg, st, svc, mgr, logger, auth.Build(policies))
	srv.Wx = wx.New(cfg.WX.AppID, cfg.WX.Secret)

	static := web.New()
	if !static.Ready() {
		logger.Warn("前端产物缺失，仅提供 API；在 web 目录执行 pnpm build 后重新编译")
	} else {
		srv.SetStatic(static)
		logger.Info("前端静态资源已挂载", "employee", "/", "admin", "/admin/")
	}

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	worker := &service.Worker{Svc: svc, Interval: time.Second}
	go worker.Run(ctx)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("HTTP 服务启动", "addr", cfg.HTTPAddr, "llm", chat.Name(),
			"vector", vec.Kind(), "bootstrap", cfg.Bootstrap)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("收到退出信号，开始优雅关闭")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("已退出")
	return nil
}

func buildModels(cfg *config.Config) (llm.Chat, llm.Embedder, error) {
	var chat llm.Chat
	switch cfg.LLM.Provider {
	case "stub":
		chat = llm.NewStubChat(cfg.LLM)
	case "openai":
		chat = llm.NewOpenAIChat(cfg.LLM)
	default:
		return nil, nil, fmt.Errorf("llm.provider 只支持 stub 或 openai，当前为 %q", cfg.LLM.Provider)
	}

	var embedder llm.Embedder
	switch cfg.Embed.Provider {
	case "stub":
		embedder = llm.NewStubEmbedder(cfg.Embed)
	case "openai":
		embedder = llm.NewOpenAIEmbedder(cfg.Embed)
	default:
		return nil, nil, fmt.Errorf("embed.provider 只支持 stub 或 openai，当前为 %q", cfg.Embed.Provider)
	}
	return chat, embedder, nil
}

// buildVectorStore 按配置构造检索实现；内存实现以 doc_embedding 为持久层，
// 启动时装载、写入时落库，重启不丢向量。
func buildVectorStore(cfg *config.Config, st *store.Store) (vector.Store, error) {
	switch cfg.Vector.Kind {
	case "pgvector":
		return vector.NewPgVector(st.Pool(), cfg.Vector.Dim), nil
	case "memory", "":
		reload := func(ctx context.Context) ([]vector.Doc, error) {
			rows, err := st.LoadEmbeddings(ctx)
			if err != nil {
				return nil, err
			}
			docs := make([]vector.Doc, 0, len(rows))
			for _, r := range rows {
				docs = append(docs, vector.Doc{
					ChunkID: r.ChunkID, DocID: r.DocID, DocTitle: r.DocTitle,
					ClauseNo: r.ClauseNo, Chapter: r.Chapter, Content: r.Content,
					Seq: r.Seq, Vec: r.Vec,
				})
			}
			return docs, nil
		}
		persist := func(ctx context.Context, docs []vector.Doc) error {
			ids := make([]int64, 0, len(docs))
			vecs := make([][]float32, 0, len(docs))
			for _, d := range docs {
				ids = append(ids, d.ChunkID)
				vecs = append(vecs, d.Vec)
			}
			return st.SaveEmbeddings(ctx, cfg.Embed.Model, ids, vecs)
		}
		return vector.NewMemory(reload, persist), nil
	default:
		return nil, fmt.Errorf("vector.kind 不支持 %q", cfg.Vector.Kind)
	}
}
