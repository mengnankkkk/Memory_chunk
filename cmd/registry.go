package main

import (
	"context-refiner/internal/engine"
	"context-refiner/internal/processor"
	"context-refiner/internal/store"
)

func buildRegistry(counter engine.TokenCounter, pageStore store.PageStore, summaryQueue store.SummaryJobQueue, pagingLimit int) *engine.Registry {
	registry := engine.NewRegistry()
	for _, item := range []engine.Processor{
		processor.NewPagingProcessor(counter, pageStore, pagingLimit),
		processor.NewCollapseProcessor(counter),
		processor.NewCompactProcessor(counter),
		processor.NewJSONTrimProcessor(counter),
		processor.NewTableReduceProcessor(counter),
		processor.NewCodeOutlineProcessor(counter),
		processor.NewErrorStackFocusProcessor(counter),
		processor.NewSnipProcessor(counter),
		processor.NewAutoCompactSyncProcessor(counter),
		processor.NewAutoCompactAsyncProcessor(counter, summaryQueue),
		processor.NewAssembleProcessor(counter),
	} {
		registry.MustRegister(item)
	}
	return registry
}
