package dto

type PageInResponse struct {
	Pages []StoredPage
}

type StoredPage struct {
	Key          string
	Content      string
	IsSummary    bool
	SummaryJobID string
}
