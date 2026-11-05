package api

import (
	"errors"
	"net/url"
	"strconv"
)

const nodePath = "v2/nodes"

// Node exposes the V2EX API 2.0 node endpoints.
type Node struct {
	Client *Client
}

func NewNode(client *Client) *Node {
	return &Node{Client: client}
}

// NodeInfo is the node object returned by the node detail endpoint.
type NodeInfo struct {
	ID           int    `json:"id"`
	FounderID    int    `json:"founder_id"`
	URL          string `json:"url"`
	Name         string `json:"name"`
	Title        string `json:"title"`
	Header       string `json:"header"`
	Footer       string `json:"footer"`
	Avatar       string `json:"avatar"`
	Topics       int    `json:"topics"`
	Created      int64  `json:"created"`
	LastModified int64  `json:"last_modified"`
}

type nodeResponse struct {
	v2Response
	Result NodeInfo `json:"result"`
}

type nodeTopicsResponse struct {
	v2Response
	Result     []TopicInfo `json:"result"`
	Pagination Pagination  `json:"pagination"`
}

// Get returns the details of a node, for example "python".
func (n *Node) Get(name string) (*NodeInfo, error) {
	if name == "" {
		return nil, errors.New("node name is required")
	}

	var body nodeResponse
	var apiErr v2Response
	resp, err := n.Client.R().
		SetResult(&body).
		SetError(&apiErr).
		Get(nodePath + "/" + url.PathEscape(name))
	if err != nil {
		return nil, err
	}
	if err := body.check(resp, apiErr); err != nil {
		return nil, err
	}

	return &body.Result, nil
}

// Topics returns a page of topics under the named node. Pages start at 1.
func (n *Node) Topics(name string, page int) ([]TopicInfo, Pagination, error) {
	if name == "" {
		return nil, Pagination{}, errors.New("node name is required")
	}
	if page < 1 {
		page = 1
	}

	var body nodeTopicsResponse
	var apiErr v2Response
	resp, err := n.Client.R().
		SetResult(&body).
		SetError(&apiErr).
		SetQueryParam("p", strconv.Itoa(page)).
		Get(nodePath + "/" + url.PathEscape(name) + "/topics")
	if err != nil {
		return nil, Pagination{}, err
	}
	if err := body.check(resp, apiErr); err != nil {
		return nil, Pagination{}, err
	}

	return body.Result, body.Pagination, nil
}
