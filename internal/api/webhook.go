package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v60/github"
	"go-pr-review/internal/domain"
	"go-pr-review/internal/service"
	"log"
	"net/http"
)

type WebHookHandler struct {
	reviewService *service.ReviewService
	chatService   *service.ChatService
	pushService   *service.PushService
	webhookSecret []byte
}

func NewWebhookHandler(service *service.ReviewService, chatService *service.ChatService, pushService *service.PushService, secret string) *WebHookHandler {
	return &WebHookHandler{
		reviewService: service,
		chatService:   chatService,
		pushService:   pushService,
		webhookSecret: []byte(secret),
	}
}

// Handler 处理/webhook请求
func (h *WebHookHandler) Handler(c *gin.Context) {
	// 1. 验证签名 (Validate Signature)
	// GitHub 会把 payload 放到 Body 里，我们需要读取它并校验 X-Hub-Signature-256
	payload, err := github.ValidatePayload(c.Request, h.webhookSecret)
	if err != nil {
		log.Printf("❌ Invalid signature: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	// 2.解析事件（Parse event)
	event, err := github.ParseWebHook(github.WebHookType(c.Request), payload)
	if err != nil {
		log.Printf("❌ Failed to parse webhook: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse webhook"})
	}

	// 3. 处理具体的事件类型
	switch e := event.(type) {
	case *github.PullRequestEvent:
		action := e.GetAction()
		// 我们只关心 PR 创建(opened) 和 代码更新(synchronize)
		if action == "opened" || action == "synchronize" {
			h.processPullRequest(e)
		}
	case *github.IssueCommentEvent:
		// 只有当是 PR 的评论，且是创建时（created）才处理
		if e.GetAction() == "created" && e.Issue.IsPullRequest() {
			// 过滤机器人自己的评论，防止无线循环！
			if e.Sender.GetType() == "Bot" {
				return
			}

			go h.chatService.HandleComment(context.Background(),
				e.Repo.Owner.GetLogin(),
				e.Repo.GetName(),
				e.Issue.GetNumber(),
				e.Comment.GetBody(),
				e.Sender.GetLogin())
		}
	case *github.PushEvent:
		// 忽略Tag推送，忽略删除分支（Deleted）
		if e.Created != nil && *e.Created {
			// New branch push? maybe review head
		}
		if e.HeadCommit != nil {
			go h.pushService.HandlePush(
				context.Background(),
				e.Repo.Owner.GetLogin(),
				e.Repo.GetName(),
				e.HeadCommit.GetID(),
			)
		}
	case *github.PingEvent:
		log.Println("🏓 Pong! GitHub webhook connected successfully.")
	default:
		// 忽略其他事件 (如 star, fork 等)
		log.Printf("ℹ️ Ignored event type: %s", github.WebHookType(c.Request))
	}

	// 无论后台处理是否成功，都要尽快返回 200 OK 给 GitHub
	c.JSON(http.StatusOK, gin.H{"status": "accepted"})
}

func (h *WebHookHandler) processPullRequest(e *github.PullRequestEvent) {
	// 提取相关信息
	owner := e.Repo.Owner.GetLogin()
	repo := e.Repo.GetName()
	prNum := e.PullRequest.GetNumber()

	log.Printf("📨 Received Webhook: %s/%s PR #%d (Action: %s)", owner, repo, prNum, e.GetAction())

	// 4. 异步触发 Review (Goroutine)
	// 因为 Review 可能需要 30秒+，不能阻塞 HTTP 响应
	go func() {
		ctx := context.Background()

		// 构造本次任务的 Config
		// 注意：Service.Run 现在的设计是接收 *domain.Config，我们需要适配一下
		// 更好的做法是 Service.Run 只接收 (ctx, owner, repo, prNum)，但为了兼容旧代码，我们临时构造一个 Config
		runConfig := &domain.Config{
			RepoOwner: owner,
			RepoName:  repo,
			PRNumber:  prNum,
			// Token 和 Key 在 main.go 初始化 Service 时已经注入到了 Provider 里，
			// 但 RepoConfig 需要在 Run 内部重新加载 (Service 已经实现了)
		}
		h.reviewService.Run(ctx, runConfig)
	}()
}
