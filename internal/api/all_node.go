package api

import (
	"errors"
	"sort"
)

// allNodesPath is a legacy V2EX API 1.0 endpoint that returns every node in
// a single unpaginated response (~800 KB, ~1400 nodes). It is absent from
// the API 2.0 documentation and may disappear; cache the result.
const allNodesPath = "nodes/all.json"

// hotNodesPerPage is how many nodes Hot returns.
const hotNodesPerPage = 20

// AllNode accesses the legacy all-nodes listing.
type AllNode struct {
	Client *Client
}

// NewAllNode returns an AllNode service backed by client.
func NewAllNode(client *Client) *AllNode {
	return &AllNode{Client: client}
}

// AllNodeInfo is one entry of the legacy all-nodes listing. It is richer
// than the API 2.0 NodeInfo: it also carries star counts and avatar sizes.
type AllNodeInfo struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	Title            string   `json:"title"`
	TitleAlternative string   `json:"title_alternative"`
	URL              string   `json:"url"`
	Topics           int      `json:"topics"`
	Stars            int      `json:"stars"`
	Header           string   `json:"header"`
	Footer           string   `json:"footer"`
	AvatarMini       string   `json:"avatar_mini"`
	AvatarNormal     string   `json:"avatar_normal"`
	AvatarLarge      string   `json:"avatar_large"`
	FounderID        int      `json:"founder_id"`
	Aliases          []string `json:"aliases"`
	Root             bool     `json:"root"`
	ParentNodeName   string   `json:"parent_node_name"`

	IsFavorite bool
}

// Get returns all V2EX nodes. The endpoint is not paginated and returns the
// full list in one response. When the client has a Cache, a fresh copy is
// served from it and a fetched copy is stored there.
func (a *AllNode) Get() ([]AllNodeInfo, error) {
	var nodes []AllNodeInfo
	if c := a.Client.Cache; c != nil {
		// The cache is best-effort: a broken file must not hide the network.
		if ok, err := c.Get(allNodesPath, &nodes); err == nil && ok {
			return nodes, nil
		}
		nodes = nil
	}

	resp, err := a.Client.R().
		SetResult(&nodes).
		Get(allNodesPath)
	if err != nil {
		return nil, err
	}

	if resp.IsError() {
		return nil, errors.New("v2ex api error: " + resp.Status())
	}

	if c := a.Client.Cache; c != nil {
		_ = c.Put(allNodesPath, nodes)
	}

	return nodes, nil
}

// Hot returns one page of the hottest nodes: the whole listing ordered by
// topic count, ties broken by stars, sliced into pages of hotNodesPerPage.
// Pages start at 1; a page below 1 is treated as 1, and a page past the end
// returns an empty slice. The listing is cached by Get, so repeated calls do
// not hit the network while it is fresh.
func (a *AllNode) Hot(page int) ([]AllNodeInfo, error) {
	if page < 1 {
		page = 1
	}

	nodes, err := a.Get()
	if err != nil {
		return nil, err
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Topics != nodes[j].Topics {
			return nodes[i].Topics > nodes[j].Topics
		}
		return nodes[i].Stars > nodes[j].Stars
	})

	start := (page - 1) * hotNodesPerPage
	if start >= len(nodes) {
		return []AllNodeInfo{}, nil
	}
	end := min(start+hotNodesPerPage, len(nodes))

	return nodes[start:end], nil
}
