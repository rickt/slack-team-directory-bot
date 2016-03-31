package fonefinder

// struct for runtime environment variables
type envs struct {
	Debug     bool
	Team      string
	Token     string
	UserToken string
}

// struct for forming a slack request
type slackRequest struct {
	Token       string `schema:"token"`
	TeamID      string `schema:"team_id"`
	TeamDomain  string `schema:"team_domain"`
	ChannelID   string `schema:"channel_id"`
	ServiceID   string `schema:"service_id"`
	ChannelName string `schema:"channel_name"`
	Timestamp   string `schema:"timestamp"`
	UserID      string `schema:"user_id"`
	UserName    string `schema:"user_name"`
	Text        string `schema:"text"`
	TriggerWord string `schema:"trigger_word"`
}

// structs for slack inbound webhook message
type Payload struct {
	Channel      string       `json:"channel"`
	Username     string       `json:"username"`
	Text         string       `json:"text"`
	ResponseType string       `json:"response_type"`
	Icon_emoji   string       `json:"icon_emoji"`
	Unfurl_links bool         `json:"unfurl_links"`
	Attachments  []Attachment `json:"attachments"`
}
type Attachment struct {
	Fallback   string  `json:"fallback"`
	Pretext    string  `json:"pretext"`
	Color      string  `json:"color"`
	AuthorName string  `json:"author_name"`
	AuthorLink string  `json:"author_link"`
	AuthorIcon string  `json:"author_icon"`
	Title      string  `json:"title"`
	TitleLink  string  `json:"title_link"`
	Text       string  `json:"text"`
	Fields     []Field `json:"fields"`
}

type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// EOF
