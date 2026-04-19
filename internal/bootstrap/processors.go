package bootstrap

import (
	"context-refiner/internal/domain/core"
	"context-refiner/internal/domain/core/processor"
	"context-refiner/internal/domain/core/repository"
)

func buildRegistry(counter core.TokenCounter, pageStore repository.PageRepository, summaryQueue repository.SummaryJobRepository, prefixStore repository.PrefixCacheRepository, prefixPolicy core.PrefixCachePolicy, pagingLimit int) *core.Registry {
	registry := core.NewRegistry()
	for _, item := range []core.Processor{
		// Stage 01: preprocess
		// 负责大块分页、重复折叠、空白压缩、富文本清洗与稳定化规范。
		processor.NewPagingProcessor(counter, pageStore, pagingLimit),
		processor.NewCollapseProcessor(counter),
		processor.NewCompactProcessor(counter),
		processor.NewSanitizeProcessor(counter),
		processor.NewCanonicalizeProcessor(counter),

		// Stage 02: transform
		// 负责按片段类型做定向裁剪与提炼，不改变整体编排，只处理局部内容形态。
		processor.NewJSONTrimProcessor(counter),
		processor.NewTableReduceProcessor(counter),
		processor.NewCodeOutlineProcessor(counter),
		processor.NewErrorStackFocusProcessor(counter),
		processor.NewSnipProcessor(counter),

		// Stage 03: compaction
		// 负责预算压力下的同步安全压缩，以及异步摘要任务投递。
		processor.NewAutoCompactSyncProcessor(counter),
		processor.NewAutoCompactAsyncProcessor(counter, summaryQueue),

		// Stage 04: finalize
		// 负责前缀缓存诊断与最终 prompt 组装，形成输出前的收尾结果。
		processor.NewPrefixCacheProcessor(counter, prefixStore, prefixPolicy),
		processor.NewAssembleProcessor(counter),
	} {
		registry.MustRegister(item)
	}
	return registry
}
