package service

import "strings"

type ExecutionStreamEmitter func(*ExecutionStreamChunk) error

type ExecutionStreamChunk struct {
	Index    int                    `json:"index"`
	TaskID   string                 `json:"task_id,omitempty"`
	Success  bool                   `json:"success"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]string      `json:"metadata,omitempty"`
	IsFinal  bool                   `json:"is_final"`
}

type executionStreamAggregator struct {
	chunkCount int
	sawFinal   bool
	lastChunk  *ExecutionStreamChunk
	result     *ExecutionResult
}

func newExecutionStreamAggregator() *executionStreamAggregator {
	return &executionStreamAggregator{}
}

func (a *executionStreamAggregator) Add(chunk *ExecutionStreamChunk) {
	if a == nil || chunk == nil {
		return
	}

	cloned := cloneExecutionStreamChunk(chunk)
	a.chunkCount++
	a.lastChunk = cloned
	if cloned.IsFinal {
		a.sawFinal = true
	}

	if a.result == nil {
		a.result = &ExecutionResult{}
	}
	a.result.TaskID = strings.TrimSpace(cloned.TaskID)
	a.result.Success = cloned.Success
	if cloned.Data != nil {
		a.result.Data = clonePayloadMap(cloned.Data)
	}
	if len(cloned.Metadata) > 0 {
		if a.result.Metadata == nil {
			a.result.Metadata = make(map[string]string, len(cloned.Metadata))
		}
		for key, value := range cloned.Metadata {
			a.result.Metadata[key] = value
		}
	}

	if trimmedErr := strings.TrimSpace(cloned.Error); trimmedErr != "" {
		a.result.Error = trimmedErr
	} else if cloned.Success || cloned.IsFinal {
		a.result.Error = ""
	}
}

func (a *executionStreamAggregator) HasChunks() bool {
	return a != nil && a.chunkCount > 0
}

func (a *executionStreamAggregator) FinalResult() *ExecutionResult {
	if a == nil || !a.HasChunks() || a.result == nil {
		return nil
	}

	return &ExecutionResult{
		TaskID:         strings.TrimSpace(a.result.TaskID),
		Success:        a.result.Success,
		Data:           clonePayloadMap(a.result.Data),
		Storage:        cloneStringMap(a.result.Storage),
		StorageChanged: a.result.StorageChanged,
		Error:          strings.TrimSpace(a.result.Error),
		Metadata:       cloneStringMap(a.result.Metadata),
	}
}

func (a *executionStreamAggregator) SyntheticFinalChunk() *ExecutionStreamChunk {
	if a == nil || !a.HasChunks() || a.sawFinal || a.lastChunk == nil {
		return nil
	}

	cloned := cloneExecutionStreamChunk(a.lastChunk)
	cloned.IsFinal = true
	return cloned
}

func cloneExecutionStreamChunk(chunk *ExecutionStreamChunk) *ExecutionStreamChunk {
	if chunk == nil {
		return nil
	}
	return &ExecutionStreamChunk{
		Index:    chunk.Index,
		TaskID:   strings.TrimSpace(chunk.TaskID),
		Success:  chunk.Success,
		Data:     clonePayloadMap(chunk.Data),
		Error:    strings.TrimSpace(chunk.Error),
		Metadata: cloneStringMap(chunk.Metadata),
		IsFinal:  chunk.IsFinal,
	}
}
