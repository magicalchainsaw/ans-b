package analytics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHotQuestionsHandlerReturnsItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/hot-questions", NewHandler(fakeHotQuestionService{
		items: []HotQuestion{{Question: "食堂几点关门？", Count: 3, Category: "餐饮服务"}},
	}).HotQuestions)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/hot-questions?limit=6", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if body := recorder.Body.String(); !containsAll(body, `"code":0`, `"question":"食堂几点关门？"`, `"count":3`) {
		t.Fatalf("unexpected response body: %s", body)
	}
}

func TestHotQuestionsHandlerRejectsInvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/hot-questions", NewHandler(fakeHotQuestionService{}).HotQuestions)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/hot-questions?limit=abc", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestHotQuestionsHandlerReturnsServiceUnavailableWhenUnconfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/hot-questions", NewHandler(fakeHotQuestionService{err: ErrRepositoryNotConfigured}).HotQuestions)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/hot-questions", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
}

type fakeHotQuestionService struct {
	items []HotQuestion
	err   error
}

func (s fakeHotQuestionService) HotQuestions(ctx context.Context, limit int) ([]HotQuestion, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

func containsAll(body string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(body, part) {
			return false
		}
	}
	return true
}
