package bootstrap

import (
	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/processor"
	"context-refiner/internal/domain/core/repository"
)

func buildRegistry(counter core.TokenCounter, pageStore repository.PageRepository, summaryQueue repository.SummaryJobRepository, prefixStore repository.PrefixCacheRepository, prefixPolicy core.PrefixCachePolicy, pagingLimit int) *core.Registry {
	registry := core.NewRegistry()
	for _, item := range []core.Processor{
		processor.NewPagingProcessor(counter, pageStore, pagingLimit),
		processor.NewCollapseProcessor(counter),
		processor.NewCompactProcessor(counter),
		processor.NewSanitizeProcessor(counter),
		processor.NewCanonicalizeProcessor(counter),
		processor.NewJSONTrimProcessor(counter),
		processor.NewTableReduceProcessor(counter),
		processor.NewCodeOutlineProcessor(counter),
		processor.NewErrorStackFocusProcessor(counter),
		processor.NewSnipProcessor(counter),
		processor.NewAutoCompactSyncProcessor(counter),
		processor.NewAutoCompactAsyncProcessor(counter, summaryQueue),
		processor.NewPrefixCacheProcessor(counter, prefixStore, prefixPolicy),
		processor.NewAssembleProcessor(counter),
	} {
		registry.MustRegister(item)
	}
	return registry
}
