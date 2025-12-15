package model

type DocPage struct {
	Title       string
	Slug        string
	Path        string
	Order       int
	Description string
	Content     string
	HTMLContent string
	Children    []*DocPage
	Parent      *DocPage
}