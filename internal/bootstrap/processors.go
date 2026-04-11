package bootstrap

import (
	"context-refiner/internal/core"
	"context-refiner/internal/core/processor"
	"context-refiner/internal/core/repository"
)

func buildRegistry(counter core.TokenCounter, pageStore repository.PageRepository, summaryQueue repository.SummaryJobRepository, pagingLimit int) *core.Registry {
	registry := core.NewRegistry()
	for _, item := range []core.Processor{
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
