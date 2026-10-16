// Package discovery preserves the hosted API using the shared catalog implementation.
package discovery

import shared "github.com/sirerun/gist/catalog/discovery"

type Authorizer = shared.Authorizer
type Searcher = shared.Searcher
type Candidate = shared.Candidate
type Request = shared.Request
type Response = shared.Response
type Service = shared.Service

var New = shared.New
