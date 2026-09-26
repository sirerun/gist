package ports

import "context"

type LexicalSearcher interface {
	Search(context.Context, SearchQuery) (SearchPage, error)
}
