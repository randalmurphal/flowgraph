package llm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/rmurphy/flowgraph/pkg/flowgraph/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockClient_FixedResponse(t *testing.T) {
	mock := llm.NewMockClient("Hello, world!")

	resp, err := mock.Complete(context.Background(), llm.CompletionRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Hi"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello, world!", resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
}

func TestMockClient_SequentialResponses(t *testing.T) {
	mock := llm.NewMockClient("").WithResponses("first", "second", "third")

	// First call
	resp, err := mock.Complete(context.Background(), llm.CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "first", resp.Content)

	// Second call
	resp, err = mock.Complete(context.Background(), llm.CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "second", resp.Content)

	// Third call
	resp, err = mock.Complete(context.Background(), llm.CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "third", resp.Content)

	// Cycles back
	resp, err = mock.Complete(context.Background(), llm.CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "first", resp.Content)
}

func TestMockClient_WithError(t *testing.T) {
	expectedErr := errors.New("test error")
	mock := llm.NewMockClient("").WithError(expectedErr)

	_, err := mock.Complete(context.Background(), llm.CompletionRequest{})
	assert.Equal(t, expectedErr, err)
}

func TestMockClient_CallTracking(t *testing.T) {
	mock := llm.NewMockClient("response")

	req1 := llm.CompletionRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "First question"}},
	}
	req2 := llm.CompletionRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Second question"}},
	}

	_, _ = mock.Complete(context.Background(), req1)
	_, _ = mock.Complete(context.Background(), req2)

	assert.Equal(t, 2, mock.CallCount())
	assert.Len(t, mock.Calls, 2)
	assert.Equal(t, "First question", mock.Calls[0].Messages[0].Content)
	assert.Equal(t, "Second question", mock.Calls[1].Messages[0].Content)
}

func TestMockClient_LastCall(t *testing.T) {
	mock := llm.NewMockClient("response")

	// No calls yet
	assert.Nil(t, mock.LastCall())

	// Make a call
	req := llm.CompletionRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Hello"}},
	}
	_, _ = mock.Complete(context.Background(), req)

	lastCall := mock.LastCall()
	require.NotNil(t, lastCall)
	assert.Equal(t, "Hello", lastCall.Messages[0].Content)
}

func TestMockClient_Reset(t *testing.T) {
	mock := llm.NewMockClient("").WithResponses("a", "b", "c")

	_, _ = mock.Complete(context.Background(), llm.CompletionRequest{})
	_, _ = mock.Complete(context.Background(), llm.CompletionRequest{})

	mock.Reset()

	assert.Equal(t, 0, mock.CallCount())
	assert.Empty(t, mock.Calls)

	// Should start from first response again
	resp, _ := mock.Complete(context.Background(), llm.CompletionRequest{})
	assert.Equal(t, "a", resp.Content)
}

func TestMockClient_CustomCompleteFunc(t *testing.T) {
	mock := llm.NewMockClient("").WithCompleteFunc(func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
		// Echo the input back
		content := req.Messages[0].Content
		return &llm.CompletionResponse{
			Content: "Echo: " + content,
		}, nil
	})

	resp, err := mock.Complete(context.Background(), llm.CompletionRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "test"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "Echo: test", resp.Content)
}

func TestMockClient_Stream(t *testing.T) {
	mock := llm.NewMockClient("streaming response")

	ch, err := mock.Stream(context.Background(), llm.CompletionRequest{})
	require.NoError(t, err)

	var chunks []llm.StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	require.Len(t, chunks, 1)
	assert.Equal(t, "streaming response", chunks[0].Content)
	assert.True(t, chunks[0].Done)
	assert.NotNil(t, chunks[0].Usage)
}

func TestMockClient_StreamWithError(t *testing.T) {
	expectedErr := errors.New("stream error")
	mock := llm.NewMockClient("").WithError(expectedErr)

	_, err := mock.Stream(context.Background(), llm.CompletionRequest{})
	assert.Equal(t, expectedErr, err)
}

func TestMockClient_ContextCancellation(t *testing.T) {
	mock := llm.NewMockClient("response")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := mock.Complete(ctx, llm.CompletionRequest{})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestMockClient_TokenUsage(t *testing.T) {
	mock := llm.NewMockClient("some response text")

	resp, err := mock.Complete(context.Background(), llm.CompletionRequest{})
	require.NoError(t, err)

	// Mock generates approximate token counts
	assert.Greater(t, resp.Usage.InputTokens, 0)
	assert.Greater(t, resp.Usage.OutputTokens, 0)
	assert.Equal(t, resp.Usage.InputTokens+resp.Usage.OutputTokens, resp.Usage.TotalTokens)
}
