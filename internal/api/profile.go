package api

// profilePath is the endpoint returning the token owner's profile.
const profilePath = "v2/member"

// Profile accesses the V2EX API 2.0 member endpoint.
type Profile struct {
	Client *Client
}

// NewProfile returns a Profile service backed by client.
func NewProfile(client *Client) *Profile {
	return &Profile{Client: client}
}

// ProfileResult is the profile of the account the token belongs to.
type ProfileResult struct {
	ID            int     `json:"id"`
	Username      string  `json:"username"`
	URL           string  `json:"url"`
	Website       string  `json:"website"`
	Twitter       *string `json:"twitter"`
	PSN           *string `json:"psn"`
	GitHub        *string `json:"github"`
	BTC           *string `json:"btc"`
	Location      string  `json:"location"`
	Tagline       string  `json:"tagline"`
	Bio           string  `json:"bio"`
	AvatarMini    string  `json:"avatar_mini"`
	AvatarNormal  string  `json:"avatar_normal"`
	AvatarLarge   string  `json:"avatar_large"`
	AvatarXLarge  string  `json:"avatar_xlarge"`
	AvatarXXLarge string  `json:"avatar_xxlarge"`
	Created       int64   `json:"created"`
	LastModified  int64   `json:"last_modified"`
	Pro           int     `json:"pro"`
}

// profileResponse is the envelope of the member endpoint.
type profileResponse struct {
	v2Response
	Result ProfileResult `json:"result"`
}

// Get returns the profile of the account the client's token belongs to.
func (p *Profile) Get() (*ProfileResult, error) {
	var body profileResponse
	var apiErr v2Response
	resp, err := p.Client.R().
		SetResult(&body).
		SetError(&apiErr).
		Get(profilePath)
	if err != nil {
		return nil, err
	}
	if err := body.check(resp, apiErr); err != nil {
		return nil, err
	}

	return &body.Result, nil
}
