package dto

type RefineRequest struct {
	SessionID   string
	RequestID   string
	Messages    []Message
	RAGChunks   []RAGChunk
	Model       Model
	TokenBudget int
	Policy      string
}

type Message struct {
	Role    string
	Content string
}

type RAGChunk struct {
	ID        string
	Source    string
	Sources   []string
	Fragments []RAGFragment
}

type RAGFragment struct {
	Type     string
	Content  string
	Language string
}

type Model struct {
	Name             string
	MaxContextTokens int
}
