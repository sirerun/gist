package webui

import (
	"encoding/json"
	"fmt"
	"strings"
)

type docLink struct{ Name, URL string }
type docSection struct {
	Heading    string
	Paragraphs []string
	Links      []docLink
	Code       string
}
type publicDocument struct {
	Title, Description string
	Sections           []docSection
}

func (d publicDocument) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n\n", d.Title, d.Description)
	for _, s := range d.Sections {
		fmt.Fprintf(&b, "## %s\n\n", s.Heading)
		for _, p := range s.Paragraphs {
			b.WriteString(p + "\n\n")
		}
		for _, l := range s.Links {
			fmt.Fprintf(&b, "- [%s](%s)\n", l.Name, l.URL)
		}
		if s.Code != "" {
			fmt.Fprintf(&b, "\n```sh\n%s\n```\n", s.Code)
		}
		b.WriteString("\n")
	}
	return b.String()
}

var publicDocuments = map[string]publicDocument{
	"/about": {Title: "About Gist and Sire Run, Inc.", Description: "Gist is an AI integrations and context library by Sire Run, Inc. It helps people and agents find useful skills, MCP servers and OpenAPI integrations for Zati and other compatible clients.", Sections: []docSection{
		{Heading: "What Gist provides", Paragraphs: []string{"The public catalog describes apps and available connection routes. Search finds relevant setup metadata; inspection and export provide a concrete descriptor a client can review. Public discovery is read-only and requires no account, API key or payment.", "Gist also provides a Go context library and an authenticated registry for private workspace artifacts. The public app inventory is broader than the set of available connectors: an app listing does not by itself promise that a client can connect to it."}},
		{Heading: "Who does the work", Paragraphs: []string{"Zati and other consuming clients own provider connections, account authorization, tool admission and execution. Gist supplies discovery and context. A retrieved skill or descriptor does not grant permission to act on someone's account."}, Links: []docLink{{"Gist source and releases", "https://github.com/sirerun/gist"}, {"Sire Run public organization profile", "https://github.com/sirerun"}, {"Developer documentation", publicOrigin + "/developers"}, {"Contact Sire Run", publicOrigin + "/contact"}}},
	}},
	"/contact": {Title: "Contact Gist · Sire Run, Inc.", Description: "Use the right channel for a Gist question. Sire Run, Inc. maintains Gist and its public catalog, source repository and releases.", Sections: []docSection{
		{Heading: "Public product questions", Paragraphs: []string{"For reproducible software problems, incorrect catalog descriptions or suggestions, open an issue in the official Gist repository. Include the affected public URL, what you expected and what happened. Public issues are visible to other readers, so remove credentials, account identifiers and private workspace content before posting."}, Links: []docLink{{"Open a Gist issue", "https://github.com/sirerun/gist/issues"}, {"Sire Run organization", "https://github.com/sirerun"}}},
		{Heading: "Private account and data questions", Paragraphs: []string{"For an invited private workspace, your workspace operator can help with access, membership and the content you have chosen to store. Do not post passwords, tokens, private instructions or mailbox contents in a public issue. Provider account authorization is handled in Zati or another consuming client; the public Gist catalog does not need your provider credentials."}, Links: []docLink{{"Data handling and privacy", publicOrigin + "/privacy"}, {"Gist setup documentation", publicOrigin + "/docs"}}},
	}},
	"/privacy": {Title: "Gist privacy and data handling", Description: "This notice describes data handled by the current Gist service operated by Sire Run, Inc. Public catalog browsing and private workspace use have different data flows.", Sections: []docSection{
		{Heading: "Public discovery", Paragraphs: []string{"The public website, REST directory and public MCP tools can be used without signing in. They return public app descriptions, connection metadata and setup links. Requests reach Gist's hosting infrastructure; operational request records can include a request identifier, route, timing, status and network information. Do not put credentials or private business information in public search queries."}},
		{Heading: "Workspace information", Paragraphs: []string{"GitHub sign-in identifies invited workspace operators. Private workspace services handle membership and authorization records, saved bookmarks, uploaded artifacts and private connection metadata according to the caller's workspace permissions. Bookmark ordering is personal to the signed-in account and is not added to public directory responses.", "A private connection entry describes setup. It is not a place to store passwords or provider access tokens. Gist does not connect to a provider on your behalf when you browse, search or export a public directory descriptor. Zati or another client manages its own provider credentials and execution."}},
		{Heading: "Your choices", Paragraphs: []string{"You can browse public content anonymously, sign out, remove bookmarks and remove private connection entries through the applicable interfaces. Ask your workspace operator about access and stored workspace content. Removing an entry from the visible interface is not a promise of immediate deletion from operational backups. This notice does not specify a retention duration or create a service-level commitment."}, Links: []docLink{{"Contact and data questions", publicOrigin + "/contact"}, {"Public source and implementation", "https://github.com/sirerun/gist"}}},
	}},
	"/pricing": {Title: "Gist public access and pricing", Description: "Public Gist catalog discovery is free to use: no sign-in, credit card, API key or contact-sales step is required.", Sections: []docSection{
		{Heading: "Start without an account", Paragraphs: []string{"Browse the app library, search public MCP/OpenAPI descriptors, inspect setup details and export metadata through the public website, REST API or Streamable HTTP MCP server. These interfaces describe integrations; they do not execute provider tools or give access to provider accounts.", "Use small pages and keep pagination cursors when continuing a search. Service concurrency and response sizes are bounded. If a request receives HTTP 429 or 503, back off rather than issuing concurrent retries. Free public access is not an unlimited throughput or availability guarantee."}, Links: []docLink{{"Public API reference", publicOrigin + "/openapi.json"}, {"Public MCP setup", publicOrigin + "/developers#public-directory"}}},
		{Heading: "Separate account and provider requirements", Paragraphs: []string{"Private Gist workspace content requires invitation and authorization. It is not necessary for public discovery. Provider accounts, model services and consuming clients can have their own terms, access requirements and charges. Review those separately before connecting or running work."}},
	}},
	"/cli": {Title: "Gist CLI and SDK", Description: "The official Gist command-line tool is available through the Sire Run Homebrew tap and public GitHub releases. The CLI indexes and searches context under your control; hosted catalog discovery is also available directly over HTTP and MCP.", Sections: []docSection{
		{Heading: "Install the official CLI", Code: "brew install sirerun/tap/gist\ngist version\ngist --help", Paragraphs: []string{"Release binaries are available for supported macOS, Linux and Windows architectures. Review the release and its checksums when installing. Go developers can also install github.com/sirerun/gist/cmd/gist with go install."}, Links: []docLink{{"Gist releases and checksums", "https://github.com/sirerun/gist/releases/latest"}, {"Official Homebrew formula", "https://github.com/sirerun/homebrew-tap/blob/main/gist.rb"}}},
		{Heading: "Choose the interface for the job", Paragraphs: []string{"The gist CLI supports local indexing, search, statistics and a stdio MCP server. Configure persistent storage for content that must survive separate CLI processes; the default in-memory store lasts only for the running process. The published local CLI is not a proxy that executes every connector in the hosted directory."}, Code: "curl -sS 'https://gist.sire.run/api/directory?q=calendar&limit=5'", Links: []docLink{{"CLI command reference", "https://github.com/sirerun/gist#cli"}, {"Hosted API schema", publicOrigin + "/openapi.json"}, {"Remote MCP setup", publicOrigin + "/developers#public-directory"}, {"Go library source", "https://github.com/sirerun/gist"}}},
	}},
}

func documentFor(path string) (publicDocument, bool) {
	d, ok := publicDocuments[path]
	if !ok {
		return d, false
	}
	// The source-controlled public identity is the only source of contact details.
	var data struct {
		Graph []struct {
			Type    string `json:"@type"`
			Contact struct {
				Email string `json:"email"`
			} `json:"contactPoint"`
		} `json:"@graph"`
	}
	if json.Unmarshal([]byte(identity()), &data) == nil {
		for _, entry := range data.Graph {
			if entry.Type == "Organization" && entry.Contact.Email != "" && (path == "/contact" || path == "/privacy") {
				d.Sections = append(append([]docSection{}, d.Sections...), docSection{Heading: "Email Sire Run", Paragraphs: []string{"Use this public contact address for support or privacy questions. Include only the information needed to identify your request; never email passwords or access tokens."}, Links: []docLink{{entry.Contact.Email, "mailto:" + entry.Contact.Email}}})
			}
		}
	}
	return d, true
}
