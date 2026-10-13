package qa

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"

	"github.com/gorag-ai/gorag/internal/admin"
)

type PromptResolver interface {
	GetPromptContent(ctx context.Context, key, fallback string) string
}

type ModelFactory func(ctx context.Context) (model.ToolCallingChatModel, error)

type Classifier struct {
	service  *Service
	prompts  PromptResolver
	newModel ModelFactory
	workers  int
	errors   *admin.AIErrorService
}

func NewClassifier(service *Service, prompts PromptResolver, newModel ModelFactory, workers int, errors *admin.AIErrorService) *Classifier {
	if workers <= 0 {
		workers = 1
	}
	return &Classifier{
		service:  service,
		prompts:  prompts,
		newModel: newModel,
		workers:  workers,
		errors:   errors,
	}
}

func (c *Classifier) Start(ctx context.Context) {
	for i := 0; i < c.workers; i++ {
		go c.loop(ctx, i)
	}
}

func (c *Classifier) loop(ctx context.Context, workerID int) {
	rdb := c.service.Redis()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		res, err := rdb.BRPop(ctx, 5*time.Second, classifyQueueKey).Result()
		if err != nil {
			if err == redis.Nil || ctx.Err() != nil {
				continue
			}
			log.Printf("[classify/%d] brpop: %v", workerID, err)
			time.Sleep(time.Second)
			continue
		}
		if len(res) < 2 {
			continue
		}
		id := res[1]
		if err := c.process(ctx, id); err != nil {
			log.Printf("[classify/%d] id=%s err=%v", workerID, id, err)
		}
	}
}

type classifyResult struct {
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

func (c *Classifier) process(ctx context.Context, id string) error {
	_ = c.service.MarkClassifyStatus(ctx, id, ClassifyProcessing, "")
	item, err := c.service.Get(ctx, id)
	if err != nil {
		return err
	}

	cats, _ := c.service.ListCategories(ctx, item.KnowledgeBaseID)
	tags, _ := c.service.ListTags(ctx, item.KnowledgeBaseID)
	tmpl := c.prompts.GetPromptContent(ctx, "qa_classify", defaultClassifyFallback)
	prompt := strings.ReplaceAll(tmpl, "{{categories}}", strings.Join(cats, ", "))
	prompt = strings.ReplaceAll(prompt, "{{tags}}", strings.Join(tags, ", "))
	prompt = strings.ReplaceAll(prompt, "{{question}}", item.Question)
	prompt = strings.ReplaceAll(prompt, "{{answer}}", item.Answer)

	chatModel, err := c.newModel(ctx)
	if err != nil {
		_ = c.service.MarkClassifyStatus(ctx, id, ClassifyFailed, err.Error())
		c.reportAIError(ctx, "qa_classify_model", err.Error(), id, "create classify model")
		return err
	}
	msg, err := chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage("你只能输出合法 JSON。"),
		schema.UserMessage(prompt),
	})
	if err != nil {
		_ = c.service.MarkClassifyStatus(ctx, id, ClassifyFailed, err.Error())
		c.reportAIError(ctx, "qa_classify_generate", err.Error(), id, prompt)
		return err
	}
	parsed, err := parseClassifyJSON(msg.Content)
	if err != nil {
		_ = c.service.MarkClassifyStatus(ctx, id, ClassifyFailed, err.Error())
		c.reportAIError(ctx, "qa_classify_parse", err.Error(), id, msg.Content)
		return err
	}
	if strings.TrimSpace(parsed.Category) == "" && len(parsed.Tags) == 0 {
		err = fmt.Errorf("empty classification result")
		_ = c.service.MarkClassifyStatus(ctx, id, ClassifyFailed, err.Error())
		c.reportAIError(ctx, "qa_classify_empty", err.Error(), id, msg.Content)
		return err
	}
	return c.service.ApplyClassification(ctx, id, parsed.Category, parsed.Tags)
}

func (c *Classifier) reportAIError(ctx context.Context, stage, message, sessionID, details string) {
	if c.errors == nil || strings.TrimSpace(message) == "" {
		return
	}
	_ = c.errors.Report(ctx, admin.AIErrorInput{
		Source:    "qa_classifier",
		Stage:     stage,
		Message:   message,
		SessionID: sessionID,
		Details:   details,
	})
}

func parseClassifyJSON(content string) (*classifyResult, error) {
	content = strings.TrimSpace(content)
	if i := strings.Index(content, "{"); i >= 0 {
		if j := strings.LastIndex(content, "}"); j > i {
			content = content[i : j+1]
		}
	}
	var out classifyResult
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	clean := make([]string, 0, len(out.Tags))
	for _, t := range out.Tags {
		t = strings.TrimSpace(t)
		if t != "" {
			clean = append(clean, t)
		}
	}
	out.Tags = clean
	out.Category = strings.TrimSpace(out.Category)
	return &out, nil
}

const defaultClassifyFallback = `你是客服知识库分类助手。根据问答内容产出 category 与 tags。
只输出 JSON：{"category":"...","tags":["..."]}

历史分类：{{categories}}
历史标签：{{tags}}

问题：{{question}}
答案：{{answer}}`
