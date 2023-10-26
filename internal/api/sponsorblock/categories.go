package sponsorblock

type Category struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	value       string `json:"value"`
}

var (
	SelfPromo         = Category{"Self Promotion", "A segment that promotes the creator's own product or service.", "selfpromo"}
	Interaction       = Category{"Interaction", "A segment that asks the viewer to interact with the video.", "interaction"}
	Sponsor           = Category{"Sponsor", "A segment that promotes a product or service.", "sponsor"}
	Preview           = Category{"Preview", "A segment that previews the video.", "preview"}
	Intro             = Category{"Intro", "A segment that introduces the video.", "intro"}
	Outro             = Category{"Outro", "A segment that concludes the video.", "outro"}
	Music             = Category{"Music", "An offtopic music segment.", "music_offtopic"}
	Filler            = Category{"Filler", "A filler segment.", "filler"}
	AvailableSegments = []Category{SelfPromo, Interaction, Sponsor, Preview, Intro, Outro, Music, Filler}
)
