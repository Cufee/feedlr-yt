package youtube

type Channel struct {
	ID          string
	Title       string
	Thumbnail   string
	Description string
}

type Video struct {
	ID          string
	URL         string
	Title       string
	Thumbnail   string
	Description string
}
