package mailtpl

//go:generate go run github.com/valyala/quicktemplate/qtc@v1.8.0 -dir=.

type MagicLinkData struct {
	AppName      string
	Title        string
	Intro        string
	Link         string
	ExpiresAt    string
	ExpiresLabel string
	ButtonText   string
	Footnote     string
}

type ApprovalData struct {
	AppName    string
	Title      string
	Greeting   string
	Intro      string
	NextStep   string
	ButtonText string
	ButtonURL  string
	Rows       []InfoRow
	Footnote   string
}

type InfoRow struct {
	Label string
	Value string
}
