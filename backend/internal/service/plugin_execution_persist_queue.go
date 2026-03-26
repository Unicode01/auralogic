package service

import (
	"log"
	"time"

	"auralogic/internal/models"
)

const (
	pluginExecutionPersistQueueSize = 1024
	pluginExecutionPersistBatchSize = 64
	pluginExecutionPersistFlushTick = 200 * time.Millisecond
)

func (s *PluginManagerService) startPluginExecutionPersistWorker(stopChan <-chan struct{}) {
	if s == nil || s.db == nil || stopChan == nil || pluginExecutionPersistQueueSize <= 0 {
		return
	}

	queue := make(chan *models.PluginExecution, pluginExecutionPersistQueueSize)

	s.executionPersistMu.Lock()
	s.executionPersistQueue = queue
	s.executionPersistMu.Unlock()

	s.executionPersistWorkerWG.Add(1)
	go func(stopSignal <-chan struct{}, persistQueue <-chan *models.PluginExecution) {
		defer s.executionPersistWorkerWG.Done()
		s.pluginExecutionPersistLoop(stopSignal, persistQueue)
	}(stopChan, queue)
}

func (s *PluginManagerService) getPluginExecutionPersistQueue() chan *models.PluginExecution {
	if s == nil {
		return nil
	}

	s.executionPersistMu.RLock()
	defer s.executionPersistMu.RUnlock()
	return s.executionPersistQueue
}

func (s *PluginManagerService) persistPluginExecutionRecord(execution *models.PluginExecution) {
	if s == nil || execution == nil {
		return
	}

	queue := s.getPluginExecutionPersistQueue()
	if queue != nil {
		select {
		case queue <- execution:
			return
		default:
		}
	}

	s.persistPluginExecutionBatch([]*models.PluginExecution{execution})
}

func (s *PluginManagerService) pluginExecutionPersistLoop(stopChan <-chan struct{}, queue <-chan *models.PluginExecution) {
	if s == nil || queue == nil {
		return
	}

	ticker := time.NewTicker(pluginExecutionPersistFlushTick)
	defer ticker.Stop()

	batch := make([]*models.PluginExecution, 0, pluginExecutionPersistBatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		s.persistPluginExecutionBatch(batch)
		batch = batch[:0]
	}

	for {
		select {
		case execution := <-queue:
			if execution == nil {
				continue
			}
			batch = append(batch, execution)
			if len(batch) >= pluginExecutionPersistBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-stopChan:
			for {
				select {
				case execution := <-queue:
					if execution == nil {
						continue
					}
					batch = append(batch, execution)
					if len(batch) >= pluginExecutionPersistBatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

func (s *PluginManagerService) persistPluginExecutionBatch(batch []*models.PluginExecution) {
	if s == nil || s.db == nil || len(batch) == 0 {
		return
	}

	if err := s.db.CreateInBatches(batch, pluginExecutionPersistBatchSize).Error; err != nil {
		for _, execution := range batch {
			if execution == nil {
				continue
			}
			if singleErr := s.db.Create(execution).Error; singleErr != nil {
				log.Printf("Failed to record plugin execution %d: %v", execution.PluginID, singleErr)
			}
		}
	}
}
