package qa

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"ans-b/server/internal/embedding"
)

type SearchRepository interface {
	SearchTop(ctx context.Context, queryEmbedding string, limit int) ([]Answer, error)
}

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

type AnswerGenerator interface {
	GenerateAnswer(ctx context.Context, question string, candidates []Answer, minScore float64) (string, error)
}

type AccessRecorder interface {
	IncrementKnowledgeAccess(ctx context.Context, itemID int64) error
	RecordQuery(ctx context.Context, userID int64, userQuestion string, matchedItemID *int64, hitScore *float64) error
}

type Service struct {
	repository     SearchRepository
	embedder       Embedder
	generator      AnswerGenerator
	accessRecorder AccessRecorder
	minScore       float64
}

type Answer struct {
	ID        int64    `json:"id"`
	ItemID    int64    `json:"item_id"`
	ChunkID   int64    `json:"chunk_id"`
	Title     string   `json:"title"`
	Question  string   `json:"matched_question"`
	Answer    string   `json:"answer"`
	ChunkText string   `json:"chunk_text"`
	SourceURL string   `json:"source_url"`
	Category  string   `json:"category"`
	Tags      []string `json:"tags"`
	Score     float64  `json:"score"`
}

type AskResult struct {
	Answered   bool     `json:"answered"`
	Answer     *Answer  `json:"answer"`
	Candidates []Answer `json:"candidates"`
	MinScore   float64  `json:"min_score"`
	AIAnswer   string   `json:"ai_answer"`
	AIEnabled  bool     `json:"ai_enabled"`
	AIError    string   `json:"ai_error"`
}

func NewService(repository SearchRepository, embedder Embedder, generators ...AnswerGenerator) *Service {
	service := &Service{
		repository: repository,
		embedder:   embedder,
		minScore:   defaultMinScore(),
	}
	if len(generators) > 0 {
		service.generator = generators[0]
	}
	return service
}

func (s *Service) SetAccessRecorder(recorder AccessRecorder) {
	s.accessRecorder = recorder
}

func (s *Service) Ask(ctx context.Context, userID int64, question string, limit int) (*AskResult, error) {
	startedAt := time.Now()
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, errors.New("question is required")
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}

	if s.embedder == nil {
		return nil, errors.New("embedder is not configured")
	}
	log.Printf("qa ask start question=%q limit=%d", question, limit)
	stageStartedAt := time.Now()
	embeddings, err := s.embedder.Embed(ctx, []string{question})
	if err != nil {
		return nil, fmt.Errorf("embed question: %w", err)
	}
	log.Printf("qa ask embedding done question=%q elapsed=%s", question, time.Since(stageStartedAt))
	if len(embeddings) != 1 {
		return nil, fmt.Errorf("embedding count mismatch: got %d, want 1", len(embeddings))
	}

	queryEmbedding := embedding.VectorLiteral(embeddings[0])
	stageStartedAt = time.Now()
	candidates, err := s.repository.SearchTop(ctx, queryEmbedding, limit)
	if err != nil {
		return nil, err
	}
	log.Printf("qa ask search done question=%q candidates=%d elapsed=%s", question, len(candidates), time.Since(stageStartedAt))
	result := &AskResult{
		Candidates: candidates,
		MinScore:   s.minScore,
	}
	if len(candidates) == 0 {
		result.Candidates = []Answer{}
		s.recordQuery(ctx, userID, question, candidates)
		log.Printf("qa ask done question=%q answered=%t ai_enabled=%t elapsed=%s", question, result.Answered, result.AIEnabled, time.Since(startedAt))
		return result, nil
	}
	result.Answered = candidates[0].Score >= s.minScore
	s.recordQuery(ctx, userID, question, candidates)
	if result.Answered {
		result.Answer = &candidates[0]
		s.incrementAccess(ctx, candidates[0].ItemID)
	}
	if result.Answered && s.generator != nil {
		result.AIEnabled = true
		stageStartedAt = time.Now()
		aiAnswer, err := s.generator.GenerateAnswer(ctx, question, candidates, s.minScore)
		if err != nil {
			result.AIError = err.Error()
			log.Printf("qa ask ai error question=%q elapsed=%s error=%v", question, time.Since(stageStartedAt), err)
			return result, nil
		}
		result.AIAnswer = strings.TrimSpace(aiAnswer)
		log.Printf("qa ask ai done question=%q elapsed=%s", question, time.Since(stageStartedAt))
	}
	log.Printf("qa ask done question=%q answered=%t ai_enabled=%t elapsed=%s", question, result.Answered, result.AIEnabled, time.Since(startedAt))
	return result, nil
}

func (s *Service) incrementAccess(ctx context.Context, itemID int64) {
	if s.accessRecorder == nil || itemID <= 0 {
		return
	}
	_ = s.accessRecorder.IncrementKnowledgeAccess(ctx, itemID)
}

func (s *Service) recordQuery(ctx context.Context, userID int64, question string, candidates []Answer) {
	if s.accessRecorder == nil {
		return
	}

	var matchedItemID *int64
	var hitScore *float64
	if len(candidates) > 0 {
		if candidates[0].ItemID > 0 {
			matchedItemID = &candidates[0].ItemID
		}
		hitScore = &candidates[0].Score
	}
	_ = s.accessRecorder.RecordQuery(ctx, userID, question, matchedItemID, hitScore)
}

func defaultMinScore() float64 {
	value := strings.TrimSpace(os.Getenv("QA_MIN_SCORE"))
	if value == "" {
		return 0.45
	}
	score, err := strconv.ParseFloat(value, 64)
	if err != nil || score < 0 || score > 1 {
		return 0.45
	}
	return score
}
