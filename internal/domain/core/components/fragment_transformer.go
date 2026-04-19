package components

type FragmentTransformReport struct {
	ChangedChunks    int
	ChangedFragments int
}

type FragmentTransformer struct{}

func NewFragmentTransformer() *FragmentTransformer {
	return &FragmentTransformer{}
}

func (t *FragmentTransformer) TransformChunks(chunks []RAGChunk, targetType string, transform func(string) string) ([]RAGChunk, FragmentTransformReport) {
	updated := make([]RAGChunk, 0, len(chunks))
	report := FragmentTransformReport{}
	for _, chunk := range chunks {
		nextChunk := chunk
		chunkChanged := false
		fragments := make([]RAGFragment, 0, len(chunk.Fragments))
		for _, fragment := range chunk.Fragments {
			nextFragment := fragment
			if fragment.Type == targetType {
				next := transform(fragment.Content)
				if next != fragment.Content {
					nextFragment.Content = next
					report.ChangedFragments++
					chunkChanged = true
				}
			}
			fragments = append(fragments, nextFragment)
		}
		nextChunk.Fragments = fragments
		if chunkChanged {
			report.ChangedChunks++
		}
		updated = append(updated, nextChunk)
	}
	return updated, report
}
